package restore

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "shared/pkg/config"
    "shared/pkg/gdrive"
    "shared/pkg/utils"
    "do-restore-service/internal/spaces"
)

type RestoreService struct {
    config       *config.DORestoreServiceConfig
    logger       *utils.Logger
    driveService *gdrive.GoogleDriveService
    spacesService *spaces.SpacesService
}

func NewRestoreService(cfg *config.DORestoreServiceConfig) (*RestoreService, error) {
    logger := utils.NewLogger("[DO-RESTORE]", cfg.Common.LogLevel)

    driveConfig := &gdrive.DriveConfig{
        CredentialsPath: cfg.GoogleDrive.CredentialsPath,
        TokenPath:       cfg.GoogleDrive.TokenPath,
        SharedDriveID:   cfg.GoogleDrive.SharedDriveID,
        FolderID:        cfg.GoogleDrive.FolderID,
    }

    driveService, err := gdrive.NewGoogleDriveService(driveConfig, logger)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize drive service: %v", err)
    }

    spacesService, err := spaces.NewSpacesService(cfg, logger)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize spaces service: %v", err)
    }

    return &RestoreService{
        config:        cfg,
        logger:        logger,
        driveService:  driveService,
        spacesService: spacesService,
    }, nil
}

func (s *RestoreService) performRestore(ctx context.Context) error {
    startTime := time.Now()
    s.logger.Info("Starting restore process...")

    // Get latest backup from Google Drive
    backup, err := s.driveService.GetLatestBackup(s.config.Restore.ContainerName)
    if err != nil {
        return fmt.Errorf("failed to get latest backup: %v", err)
    }

    s.logger.Info("Found latest backup: %s (Created: %s, Size: %s)",
        backup.Name,
        backup.CreatedTime.Format("2006-01-02 15:04:05"),
        utils.FormatBytes(backup.Size))

    // Create temp directory
    tempDir := filepath.Join(s.config.Restore.TempDir, fmt.Sprintf("restore_%s_%s",
        s.config.Restore.ContainerName,
        time.Now().Format("20060102_150405")))
    if err := os.MkdirAll(tempDir, 0755); err != nil {
        return fmt.Errorf("failed to create temp directory: %v", err)
    }
    defer os.RemoveAll(tempDir)

    // Download backup from Google Drive
    s.logger.Info("Downloading backup file...")
    zipPath := filepath.Join(tempDir, backup.Name)
    if err := s.driveService.DownloadFile(ctx, backup.ID, zipPath); err != nil {
        return fmt.Errorf("failed to download backup: %v", err)
    }

    // Extract backup
    s.logger.Info("Extracting backup archive...")
    extractPath := filepath.Join(tempDir, "extracted")
    if err := utils.UnzipFile(zipPath, extractPath); err != nil {
        return fmt.Errorf("failed to extract backup: %v", err)
    }

    // Delete existing files in Spaces (optional, based on your needs)
    s.logger.Info("Cleaning up existing files in Spaces...")
    if err := s.spacesService.DeletePrefix(ctx, s.config.Restore.ContainerName); err != nil {
        s.logger.Warn("Failed to cleanup existing files: %v", err)
    }

    // Upload to Spaces
    s.logger.Info("Uploading files to Spaces...")
    stats, err := s.spacesService.UploadFiles(ctx, extractPath, s.config.Restore.ContainerName)
    if err != nil {
        return fmt.Errorf("failed to upload to spaces: %v", err)
    }

    duration := time.Since(startTime)
    s.logger.Info("Restore completed in %v:", duration)
    s.logger.Info("- Files processed: %d", stats.FilesCount)
    s.logger.Info("- Total size: %.2f MB", float64(stats.TotalSize)/(1024*1024))
    s.logger.Info("- Average speed: %.2f MB/s", float64(stats.TotalSize)/(1024*1024)/duration.Seconds())

    return nil
}

func (s *RestoreService) RunOnce(ctx context.Context) error {
    return s.performRestore(ctx)
}