package backup

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/robfig/cron/v3"
    "shared/pkg/config"
    "shared/pkg/utils"
)

type BackupService struct {
    config       *config.BackupServiceConfig
    logger       *utils.Logger
    azureService *AzureService
    driveService *GoogleDriveBackup
}

func NewBackupService(cfg *config.BackupServiceConfig) (*BackupService, error) {
    logger := utils.NewLogger("[BACKUP]", cfg.Common.LogLevel)

    azureService, err := NewAzureService(cfg, logger)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize azure service: %v", err)
    }

    driveService, err := NewGoogleDriveBackup(cfg, logger)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize drive service: %v", err)
    }

    return &BackupService{
        config:       cfg,
        logger:       logger,
        azureService: azureService,
        driveService: driveService,
    }, nil
}

func (s *BackupService) performBackup(ctx context.Context) error {
    startTime := time.Now()
    s.logger.Info("Starting backup process...")

    // Create backup root directory if not exists
    backupRootDir := s.config.Backup.BackupPath
    if err := os.MkdirAll(backupRootDir, 0755); err != nil {
        return fmt.Errorf("failed to create backup directory: %v", err)
    }

    // Download/sync from Azure
    stats, err := s.azureService.DownloadBlobs(ctx, backupRootDir)
    if err != nil {
        return fmt.Errorf("azure download failed: %v", err)
    }

    // Create zip file for each container that had changes
    var totalSize int64
    for containerName, containerStats := range stats {
        if containerStats.DownloadedFiles > 0 {
            // Create zip file
            containerDir := filepath.Join(backupRootDir, containerName)
            timestamp := time.Now().Format("20060102_150405")
            zipPath := filepath.Join(s.config.Backup.TempDir,
                fmt.Sprintf("%s_%s.zip", containerName, timestamp))

            s.logger.Info("Creating backup archive for %s...", containerName)
            if err := utils.ZipDirectory(containerDir, zipPath); err != nil {
                s.logger.Error("Failed to create zip for %s: %v", containerName, err)
                continue
            }

            // Upload to Google Drive
            s.logger.Info("Uploading %s to Google Drive...", containerName)
            if err := s.driveService.UploadBackup(ctx, zipPath, containerName); err != nil {
                s.logger.Error("Failed to upload %s: %v", containerName, err)
                os.Remove(zipPath)
                continue
            }

            // Cleanup temp zip file
            os.Remove(zipPath)
            totalSize += containerStats.TotalSize
        }
    }

    // Cleanup old backups from Google Drive
    if err := s.driveService.CleanupOldBackups(ctx, s.config.Backup.RetentionDays); err != nil {
        s.logger.Error("Failed to cleanup old backups: %v", err)
    }

    duration := time.Since(startTime)
    s.logger.Info("Backup completed in %v", duration)
    s.logger.Info("Total containers processed: %d", len(stats))
    s.logger.Info("Total size: %.2f MB", float64(totalSize)/(1024*1024))

    return nil
}

func (s *BackupService) StartScheduler() error {
    c := cron.New(cron.WithLocation(s.config.Backup.TimeZone))

    _, err := c.AddFunc(s.config.Backup.Schedule, func() {
        ctx := context.Background()
        if err := s.performBackup(ctx); err != nil {
            s.logger.Error("Backup failed: %v", err)
        }
    })

    if err != nil {
        return fmt.Errorf("failed to schedule backup: %v", err)
    }

    c.Start()
    s.logger.Info("Backup scheduler started with schedule: %s", s.config.Backup.Schedule)
    s.logger.Info("Next backup scheduled for: %s",
        c.Entries()[0].Schedule.Next(time.Now()).Format("2006-01-02 15:04:05"))

    return nil
}

func (s *BackupService) ListFolders() error {
    return s.driveService.ListAvailableFolders()
}