package restore

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "restore-service/internal/config"
    "restore-service/internal/utils"
)

type RestoreService struct {
    config       *config.Config
    logger       *utils.Logger
    driveService *GoogleDriveService
    azureService *AzureService
}

func NewRestoreService(cfg *config.Config) (*RestoreService, error) {
    logger := utils.NewLogger("[RESTORE]", cfg.LogLevel)

    driveService, err := NewGoogleDriveService(cfg, logger)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize drive service: %v", err)
    }

    azureService, err := NewAzureService(cfg, logger)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize azure service: %v", err)
    }

    return &RestoreService{
        config:       cfg,
        logger:       logger,
        driveService: driveService,
        azureService: azureService,
    }, nil
}

func (s *RestoreService) RestoreLatest(ctx context.Context) error {
    s.logger.Info("Starting restore process from latest backup...")

    // Get latest backup file
    backup, err := s.driveService.GetLatestBackup()
    if err != nil {
        return fmt.Errorf("failed to get latest backup: %v", err)
    }

    return s.restore(ctx, backup)
}

func (s *RestoreService) RestoreFromDate(ctx context.Context, date time.Time) error {
    s.logger.Info("Starting restore process from backup date: %s", date.Format("2006-01-02"))

    // Get backup file from specific date
    backup, err := s.driveService.GetBackupFromDate(date)
    if err != nil {
        return fmt.Errorf("failed to get backup from date %s: %v", date.Format("2006-01-02"), err)
    }

    return s.restore(ctx, backup)
}

func (s *RestoreService) restore(ctx context.Context, backup *DriveBackup) error {
    startTime := time.Now()
    s.logger.Info("Using backup: %s (Created: %s)", backup.Name, backup.CreatedTime)

    // Create temp directory for restore
    tempDir := filepath.Join(s.config.Restore.TempDir, fmt.Sprintf("restore_%s", time.Now().Format("20060102_150405")))
    if err := os.MkdirAll(tempDir, 0755); err != nil {
        return fmt.Errorf("failed to create temp directory: %v", err)
    }
    defer os.RemoveAll(tempDir) // Cleanup when done

    // Download backup file
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

    // Upload to Azure
    s.logger.Info("Uploading files to Azure Storage...")
    if err := s.azureService.UploadFiles(ctx, extractPath); err != nil {
        return fmt.Errorf("failed to upload to azure: %v", err)
    }

    duration := time.Since(startTime)
    s.logger.Info("Restore completed successfully in %v", duration)
    return nil
}