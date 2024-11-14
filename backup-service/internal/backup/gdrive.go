package backup

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
    "google.golang.org/api/drive/v3"
    "google.golang.org/api/option"

    "backup-service/internal/config"
    "backup-service/internal/utils"
)

type GoogleDriveService struct {
    service *drive.Service
    config  *config.Config
    logger  *utils.Logger
}

func NewGoogleDriveService(cfg *config.Config, logger *utils.Logger) (*GoogleDriveService, error) {
    ctx := context.Background()

    b, err := os.ReadFile(cfg.GoogleDrive.CredentialsPath)
    if err != nil {
        return nil, fmt.Errorf("unable to read credentials file: %v", err)
    }

    // Configure Google Drive API with full access scope
    config, err := google.ConfigFromJSON(b, drive.DriveScope)
    if err != nil {
        return nil, fmt.Errorf("unable to parse credentials: %v", err)
    }

    token, err := loadToken(cfg.GoogleDrive.TokenPath)
    if err != nil {
        return nil, fmt.Errorf("unable to load token: %v", err)
    }

    service, err := drive.NewService(ctx,
        option.WithTokenSource(config.TokenSource(ctx, token)))
    if err != nil {
        return nil, fmt.Errorf("unable to create drive service: %v", err)
    }

    drive, err := s.service.Drives.Get(cfg.GoogleDrive.SharedDriveID).Do()
    if err != nil {
        return nil, fmt.Errorf("failed to access shared drive: %v", err)
    }
    s.logger.Info("Successfully connected to Shared Drive: %s", drive.Name)

    return &GoogleDriveService{
        service: service,
        config:  cfg,
        logger:  logger,
    }, nil
}

func loadToken(path string) (*oauth2.Token, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    token := &oauth2.Token{}
    err = json.NewDecoder(f).Decode(token)
    return token, err
}

func (s *GoogleDriveService) UploadBackup(ctx context.Context, zipPath string, containerName string) error {
    // Get file info for progress tracking
    fileInfo, err := os.Stat(zipPath)
    if err != nil {
        return fmt.Errorf("failed to get file info: %v", err)
    }

    // Create or get container folder in Google Drive
    folderId, err := s.getOrCreateContainerFolder(containerName)
    if err != nil {
        return fmt.Errorf("failed to get/create container folder: %v", err)
    }

    // Create backup folder with timestamp
    backupFolder, err := s.createFolder(
        fmt.Sprintf("backup_%s", time.Now().Format("20060102_150405")),
        []string{folderId},
    )
    if err != nil {
        return fmt.Errorf("failed to create backup folder: %v", err)
    }

    // Open and upload zip file
    file, err := os.Open(zipPath)
    if err != nil {
        return fmt.Errorf("failed to open zip file: %v", err)
    }
    defer file.Close()

    // Create file metadata
    zipFile := &drive.File{
        Name:     filepath.Base(zipPath),
        Parents:  []string{backupFolder.Id},
    }

    // Upload with progress tracking
    uploadStart := time.Now()
    s.logger.Info("Starting upload of %s (%.2f MB)", filepath.Base(zipPath), float64(fileInfo.Size())/(1024*1024))

    result, err := s.service.Files.Create(zipFile).
        Media(file).
        SupportsAllDrives(true).
        Do()

    if err != nil {
        return fmt.Errorf("upload failed: %v", err)
    }

    duration := time.Since(uploadStart)
    speed := float64(fileInfo.Size()) / duration.Seconds() / 1024 / 1024 // MB/s

    s.logger.Info("Upload completed: %s (%.2f MB/s)", result.Name, speed)
    return nil
}

func (s *GoogleDriveService) getOrCreateContainerFolder(containerName string) (string, error) {
    // Verify Shared Drive access first
    drive, err := s.service.Drives.Get(s.config.GoogleDrive.SharedDriveID).Do()
    if err != nil {
        return "", fmt.Errorf("failed to access shared drive: %v", err)
    }
    s.logger.Info("Using Shared Drive: %s", drive.Name)

    // Search for existing container folder
    query := fmt.Sprintf("name='%s' and mimeType='application/vnd.google-apps.folder' and '%s' in parents and trashed=false",
        containerName, s.config.GoogleDrive.SharedDriveID)

    fileList, err := s.service.Files.List().
        Q(query).
        SupportsAllDrives(true).
        IncludeItemsFromAllDrives(true).
        Corpora("drive").
        DriveId(s.config.GoogleDrive.SharedDriveID).
        Fields("files(id, name)").
        Do()

    if err != nil {
        return "", fmt.Errorf("failed to search for container folder: %v", err)
    }

    // Return existing folder if found
    if len(fileList.Files) > 0 {
        s.logger.Info("Found existing container folder: %s", fileList.Files[0].Name)
        return fileList.Files[0].Id, nil
    }

    // Create new container folder
    s.logger.Info("Creating new container folder: %s", containerName)
    folder, err := s.createFolder(containerName, []string{s.config.GoogleDrive.SharedDriveID})
    if err != nil {
        return "", err
    }

    return folder.Id, nil
}

func (s *GoogleDriveService) createFolder(name string, parents []string) (*drive.File, error) {
    folder := &drive.File{
        Name:     name,
        MimeType: "application/vnd.google-apps.folder",
        Parents:  parents,
    }

    result, err := s.service.Files.Create(folder).
        SupportsAllDrives(true).
        Fields("id, name").
        Do()

    if err != nil {
        return nil, fmt.Errorf("failed to create folder: %v", err)
    }

    return result, nil
}

func (s *GoogleDriveService) CleanupOldBackups(ctx context.Context, retentionDays int) error {
    cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

    // List all container folders in shared drive
    containerFolders, err := s.service.Files.List().
        Q(fmt.Sprintf("mimeType='application/vnd.google-apps.folder' and '%s' in parents and trashed=false",
            s.config.GoogleDrive.SharedDriveID)).
        SupportsAllDrives(true).
        IncludeItemsFromAllDrives(true).
        Corpora("drive").
        DriveId(s.config.GoogleDrive.SharedDriveID).
        Fields("files(id, name)").
        Do()

    if err != nil {
        return fmt.Errorf("failed to list container folders: %v", err)
    }

    for _, containerFolder := range containerFolders.Files {
        // List backup folders in each container
        query := fmt.Sprintf(
            "mimeType='application/vnd.google-apps.folder' and name contains 'backup_' and '%s' in parents and createdTime < '%s' and trashed=false",
            containerFolder.Id,
            cutoffTime.Format(time.RFC3339))

        backupFolders, err := s.service.Files.List().
            Q(query).
            SupportsAllDrives(true).
            IncludeItemsFromAllDrives(true).
            Corpora("drive").
            DriveId(s.config.GoogleDrive.SharedDriveID).
            Fields("files(id, name, createdTime)").
            Do()

        if err != nil {
            s.logger.Error("Failed to list backup folders for container %s: %v", containerFolder.Name, err)
            continue
        }

        // Delete old backup folders
        for _, folder := range backupFolders.Files {
            err := s.service.Files.Delete(folder.Id).
                SupportsAllDrives(true).
                Do()

            if err != nil {
                s.logger.Error("Failed to delete old backup %s: %v", folder.Name, err)
                continue
            }
            s.logger.Info("Deleted old backup: %s", folder.Name)
        }
    }

    return nil
}

// Helper function to format bytes
func formatBytes(bytes int64) string {
    const unit = 1024
    if bytes < unit {
        return fmt.Sprintf("%d B", bytes)
    }
    div, exp := int64(unit), 0
    for n := bytes / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}