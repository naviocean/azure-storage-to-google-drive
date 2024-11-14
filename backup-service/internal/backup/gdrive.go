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

    // List all available Shared Drives
    drives, err := service.Drives.List().PageSize(10).Do()
    if err != nil {
        logger.Error("Unable to list drives: %v", err)
    } else {
        logger.Info("Available Shared Drives:")
        for _, d := range drives.Drives {
            logger.Info("- Name: %s, ID: %s", d.Name, d.Id)
        }
    }

    // Verify shared drive access
    if _, err := service.Drives.Get(cfg.GoogleDrive.SharedDriveID).Do(); err != nil {
        return nil, fmt.Errorf("failed to access shared drive: %v", err)
    }

    // If folder ID is specified, verify it exists and is accessible
    if cfg.GoogleDrive.FolderID != "" {
        folder, err := service.Files.Get(cfg.GoogleDrive.FolderID).
            SupportsAllDrives(true).
            Fields("id, name, mimeType, parents").
            Do()
        if err != nil {
            return nil, fmt.Errorf("failed to access specified folder: %v", err)
        }

        // Verify it's a folder
        if folder.MimeType != "application/vnd.google-apps.folder" {
            return nil, fmt.Errorf("specified ID is not a folder")
        }

        // Verify it's in the correct shared drive
        var inSharedDrive bool
        for _, parent := range folder.Parents {
            if parent == cfg.GoogleDrive.SharedDriveID {
                inSharedDrive = true
                break
            }
        }
        if !inSharedDrive {
            return nil, fmt.Errorf("specified folder is not in the configured shared drive")
        }

        logger.Info("Using folder: %s", folder.Name)
    }

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
    parentID := s.config.GoogleDrive.SharedDriveID
    if s.config.GoogleDrive.FolderID != "" {
        parentID = s.config.GoogleDrive.FolderID
    }

    // Search for existing container folder
    query := fmt.Sprintf("name='%s' and mimeType='application/vnd.google-apps.folder' and '%s' in parents and trashed=false",
        containerName, parentID)

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
        return fileList.Files[0].Id, nil
    }

    // Create new container folder
    folder := &drive.File{
        Name:     containerName,
        MimeType: "application/vnd.google-apps.folder",
        Parents:  []string{parentID},
    }

    result, err := s.service.Files.Create(folder).
        SupportsAllDrives(true).
        Fields("id, name").
        Do()

    if err != nil {
        return "", fmt.Errorf("failed to create folder: %v", err)
    }

    return result.Id, nil
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

    // Determine parent folder ID
    parentID := s.config.GoogleDrive.SharedDriveID
    if s.config.GoogleDrive.FolderID != "" {
        parentID = s.config.GoogleDrive.FolderID
    }

    // List all container folders in the specified parent
    containerFolders, err := s.service.Files.List().
        Q(fmt.Sprintf("mimeType='application/vnd.google-apps.folder' and '%s' in parents and trashed=false",
            parentID)).
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

// Helper function to list available folders
func (s *GoogleDriveService) ListAvailableFolders() error {
    query := fmt.Sprintf("mimeType='application/vnd.google-apps.folder' and '%s' in parents and trashed=false",
        s.config.GoogleDrive.SharedDriveID)

    fileList, err := s.service.Files.List().
        Q(query).
        SupportsAllDrives(true).
        IncludeItemsFromAllDrives(true).
        Corpora("drive").
        DriveId(s.config.GoogleDrive.SharedDriveID).
        Fields("files(id, name, createdTime)").
        Do()

    if err != nil {
        return fmt.Errorf("failed to list folders: %v", err)
    }

    s.logger.Info("Available folders in Shared Drive:")
    for _, folder := range fileList.Files {
        s.logger.Info("- Name: %s, ID: %s", folder.Name, folder.Id)
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