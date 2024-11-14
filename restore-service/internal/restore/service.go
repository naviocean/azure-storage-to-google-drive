package restore

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    "backup-service/internal/config"
    "backup-service/internal/utils"
)

type RestoreService struct {
    config       *config.Config
    logger       *utils.Logger
    driveService *GoogleDriveService
    azureService *AzureService
}

func (s *RestoreService) RestoreLatest(ctx context.Context) error {
    if s.config.Azure.ContainerName == "ALL" {
        return s.restoreAllContainers(ctx, nil)
    }
    return s.restoreContainer(ctx, s.config.Azure.ContainerName, nil)
}

func (s *RestoreService) RestoreFromDate(ctx context.Context, date time.Time) error {
    if s.config.Azure.ContainerName == "ALL" {
        return s.restoreAllContainers(ctx, &date)
    }
    return s.restoreContainer(ctx, s.config.Azure.ContainerName, &date)
}

func (s *RestoreService) restoreAllContainers(ctx context.Context, date *time.Time) error {
    // Get available backups for all containers
    backups, err := s.driveService.ListAvailableBackups()
    if err != nil {
        return fmt.Errorf("failed to list backups: %v", err)
    }

    // Group backups by container
    containerBackups := make(map[string][]*DriveBackup)
    for _, backup := range backups {
        // Parse container name from backup file name
        // Example: container1_backup_20231114_120000.zip
        parts := strings.Split(backup.Name, "_backup_")
        if len(parts) != 2 {
            s.logger.Warn("Invalid backup file name format: %s", backup.Name)
            continue
        }
        containerName := parts[0]
        containerBackups[containerName] = append(containerBackups[containerName], backup)
    }

    // Process each container
    for containerName, backups := range containerBackups {
        if len(backups) == 0 {
            s.logger.Warn("No backups found for container: %s", containerName)
            continue
        }

        var backupToRestore *DriveBackup
        if date != nil {
            // Find backup closest to specified date
            backupToRestore = findClosestBackup(backups, *date)
            if backupToRestore == nil {
                s.logger.Warn("No backup found for container %s on date %s", containerName, date.Format("2006-01-02"))
                continue
            }
        } else {
            // Use latest backup
            backupToRestore = backups[0] // Already sorted by date desc
        }

        s.logger.Info("Restoring container %s from backup: %s", containerName, backupToRestore.Name)
        if err := s.processRestore(ctx, containerName, backupToRestore); err != nil {
            s.logger.Error("Failed to restore container %s: %v", containerName, err)
            continue
        }
    }

    return nil
}

func (s *RestoreService) restoreContainer(ctx context.Context, containerName string, date *time.Time) error {
    var backup *DriveBackup
    var err error

    if date != nil {
        backup, err = s.driveService.GetBackupFromDate(*date, containerName)
    } else {
        backup, err = s.driveService.GetLatestBackup(containerName)
    }

    if err != nil {
        return fmt.Errorf("failed to get backup: %v", err)
    }

    return s.processRestore(ctx, containerName, backup)
}

func (s *RestoreService) processRestore(ctx context.Context, containerName string, backup *DriveBackup) error {
    startTime := time.Now()
    s.logger.Info("Starting restore process for container: %s", containerName)
    s.logger.Info("Using backup: %s (Created: %s, Size: %.2f MB)",
        backup.Name,
        backup.CreatedTime.Format("2006-01-02 15:04:05"),
        float64(backup.Size)/(1024*1024))

    // Create temp directory for restore
    tempDir := filepath.Join(s.config.Restore.TempDir, fmt.Sprintf("restore_%s_%s",
        containerName,
        time.Now().Format("20060102_150405")))
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
    stats, err := s.azureService.UploadFiles(ctx, extractPath, containerName)
    if err != nil {
        return fmt.Errorf("failed to upload to azure: %v", err)
    }

    duration := time.Since(startTime)
    s.logger.Info("Restore completed for container %s in %v:", containerName, duration)
    s.logger.Info("- Files processed: %d", stats.FilesCount)
    s.logger.Info("- Total size: %.2f MB", float64(stats.TotalSize)/(1024*1024))
    s.logger.Info("- Average speed: %.2f MB/s", float64(stats.TotalSize)/(1024*1024)/duration.Seconds())

    return nil
}

// Helper function to find backup closest to specified date
func findClosestBackup(backups []*DriveBackup, targetDate time.Time) *DriveBackup {
    targetDate = time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())

    var closest *DriveBackup
    var minDiff time.Duration
    for _, backup := range backups {
        diff := backup.CreatedTime.Sub(targetDate)
        if diff < 0 {
            diff = -diff
        }
        if closest == nil || diff < minDiff {
            closest = backup
            minDiff = diff
        }
    }
    return closest
}