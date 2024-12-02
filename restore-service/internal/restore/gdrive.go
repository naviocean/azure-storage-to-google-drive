package restore

import (
    "context"
    "time"

    "shared/pkg/config"
    "shared/pkg/gdrive"
    "shared/pkg/utils"
)

type GoogleDriveRestore struct {
    service *gdrive.GoogleDriveService
    config  *config.RestoreServiceConfig
    logger  *utils.Logger
}

func NewGoogleDriveRestore(cfg *config.RestoreServiceConfig, logger *utils.Logger) (*GoogleDriveRestore, error) {
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

    return &GoogleDriveRestore{
        service: service,
        config:  cfg,
        logger:  logger,
    }, nil
}

func (r *GoogleDriveRestore) ListAvailableBackups() ([]*gdrive.DriveBackup, error) {
    return r.service.ListAvailableBackups()
}

func (r *GoogleDriveRestore) GetLatestBackup(containerName string) (*gdrive.DriveBackup, error) {
    return r.service.GetLatestBackup(containerName)
}

func (r *GoogleDriveRestore) GetBackupFromDate(date time.Time, containerName string) (*gdrive.DriveBackup, error) {
    return r.service.GetBackupFromDate(date, containerName)
}

func (r *GoogleDriveRestore) DownloadFile(ctx context.Context, fileID string, destinationPath string) error {
    return r.service.DownloadFile(ctx, fileID, destinationPath)
}