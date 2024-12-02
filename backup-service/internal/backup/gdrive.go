package backup

import (
    "context"

    "shared/pkg/config"
    "shared/pkg/gdrive"
    "shared/pkg/utils"
)

type GoogleDriveBackup struct {
    service *gdrive.GoogleDriveService
    config  *config.BackupServiceConfig
    logger  *utils.Logger
}

func NewGoogleDriveBackup(cfg *config.BackupServiceConfig, logger *utils.Logger) (*GoogleDriveBackup, error) {
    driveConfig := &gdrive.DriveConfig{
        CredentialsPath: cfg.GoogleDrive.CredentialsPath,
        TokenPath:       cfg.GoogleDrive.TokenPath,
        SharedDriveID:   cfg.GoogleDrive.SharedDriveID,
        FolderID:        cfg.GoogleDrive.FolderID,
    }

    service, err := gdrive.NewGoogleDriveService(driveConfig, logger)
    if err != nil {
        return nil, err
    }

    return &GoogleDriveBackup{
        service: service,
        config:  cfg,
        logger:  logger,
    }, nil
}

func (b *GoogleDriveBackup) UploadBackup(ctx context.Context, zipPath string, containerName string) error {
    return b.service.UploadBackup(ctx, zipPath, containerName)
}

func (b *GoogleDriveBackup) CleanupOldBackups(ctx context.Context, retentionDays int) error {
    return b.service.CleanupOldBackups(ctx, retentionDays)
}

func (b *GoogleDriveBackup) ListAvailableFolders() error {
    return b.service.ListAvailableFolders()
}