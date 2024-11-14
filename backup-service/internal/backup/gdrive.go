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

    "shared/pkg/config"
    "shared/pkg/utils"
)

type GoogleDriveService struct {
    service *drive.Service
    config  *config.BackupServiceConfig
    logger  *utils.Logger
}

func NewGoogleDriveService(cfg *config.BackupServiceConfig, logger *utils.Logger) (*GoogleDriveService, error) {
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

    // Verify Shared Drive access
    drive, err := service.Drives.Get(cfg.GoogleDrive.SharedDriveID).Do()
    if err != nil {
        return nil, fmt.Errorf("failed to access shared drive: %v", err)
    }
    logger.Info("Connected to Shared Drive: %s", drive.Name)

    // Verify folder access if specified
    if cfg.GoogleDrive.FolderID != "" {
        folder, err := service.Files.Get(cfg.GoogleDrive.FolderID).
            SupportsAllDrives(true).
            Fields("id, name, parents").
            Do()
        if err != nil {
            return nil, fmt.Errorf("failed to access specified folder: %v", err)
        }

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

    // Create folder name with timestamp
    folderName := fmt.Sprintf("backup_%s_%s", containerName, time.Now().Format("20060102_150405"))

    // Create folder in Drive
    folder := &drive.File{
        Name:     folderName,
        MimeType: "application/vnd.google-apps.folder",
    }

    if s.config.GoogleDrive.SharedDriveID != "" {
        folder.Parents = []string{s.config.GoogleDrive.SharedDriveID}
        if s.config.GoogleDrive.FolderID != "" {
            folder.Parents = []string{s.config.GoogleDrive.FolderID}
        }
    }

    createdFolder, err := s.service.Files.Create(folder).
        SupportsAllDrives(true).
        Fields("id, name").
        Do()
    if err != nil {
        return fmt.Errorf("failed to create folder: %v", err)
    }

    // Upload zip file
    file, err := os.Open(zipPath)
    if err != nil {
        return fmt.Errorf("failed to open zip file: %v", err)
    }
    defer file.Close()

    fileInfo, err := file.Stat()
    if err != nil {
        return fmt.Errorf("failed to get file info: %v", err)
    }

    zipFile := &drive.File{
        Name:    filepath.Base(zipPath),
        Parents: []string{createdFolder.Id},
    }

    startTime := time.Now()
    s.logger.Info("Starting upload of %s (%s)", filepath.Base(zipPath), formatBytes(fileInfo.Size()))

    result, err := s.service.Files.Create(zipFile).
        Media(file).
        SupportsAllDrives(true).
        Do()
    if err != nil {
        return fmt.Errorf("upload failed: %v", err)
    }

    duration := time.Since(startTime)
    speed := float64(fileInfo.Size()) / duration.Seconds() / 1024 / 1024 // MB/s

    s.logger.Info("Upload completed: %s (%s, %.2f MB/s)",
        result.Name,
        formatBytes(fileInfo.Size()),
        speed)
    return nil
}

func (s *GoogleDriveService) CleanupOldBackups(ctx context.Context, retentionDays int) error {
    cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

    query := fmt.Sprintf(
        "mimeType='application/vnd.google-apps.folder' and name contains 'backup_' "+
        "and createdTime < '%s' and trashed=false",
        cutoffTime.Format(time.RFC3339),
    )

    var fileList *drive.FileList
    var err error

    if s.config.GoogleDrive.SharedDriveID != "" {
        fileList, err = s.service.Files.List().
            Q(query).
            SupportsAllDrives(true).
            IncludeItemsFromAllDrives(true).
            Corpora("drive").
            DriveId(s.config.GoogleDrive.SharedDriveID).
            Fields("files(id, name, createdTime)").
            Do()
    } else {
        fileList, err = s.service.Files.List().
            Q(query).
            Fields("files(id, name, createdTime)").
            Do()
    }

    if err != nil {
        return fmt.Errorf("failed to list old backups: %v", err)
    }

    for _, file := range fileList.Files {
        var err error
        if s.config.GoogleDrive.SharedDriveID != "" {
            err = s.service.Files.Delete(file.Id).
                SupportsAllDrives(true).
                Do()
        } else {
            err = s.service.Files.Delete(file.Id).Do()
        }

        if err != nil {
            s.logger.Error("Failed to delete old backup %s: %v", file.Name, err)
            continue
        }
        s.logger.Info("Deleted old backup: %s", file.Name)
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