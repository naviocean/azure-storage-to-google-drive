package gdrive

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"
    "time"

    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
    "google.golang.org/api/drive/v3"
    "google.golang.org/api/option"

    "shared/pkg/utils"
)

type DriveConfig struct {
    CredentialsPath string
    TokenPath       string
    SharedDriveID   string
    FolderID        string
}

type DriveBackup struct {
    ID          string
    Name        string
    CreatedTime time.Time
    Size        int64
}

type GoogleDriveService struct {
    service *drive.Service
    config  *DriveConfig
    logger  *utils.Logger
}

func NewGoogleDriveService(cfg *DriveConfig, logger *utils.Logger) (*GoogleDriveService, error) {
    ctx := context.Background()

    b, err := os.ReadFile(cfg.CredentialsPath)
    if err != nil {
        return nil, fmt.Errorf("unable to read credentials file: %v", err)
    }

    config, err := google.ConfigFromJSON(b, drive.DriveScope)
    if err != nil {
        return nil, fmt.Errorf("unable to parse credentials: %v", err)
    }

    token, err := loadToken(cfg.TokenPath)
    if err != nil {
        return nil, fmt.Errorf("unable to load token: %v", err)
    }

    service, err := drive.NewService(ctx,
        option.WithTokenSource(config.TokenSource(ctx, token)))
    if err != nil {
        return nil, fmt.Errorf("unable to create drive service: %v", err)
    }

    // Verify Shared Drive access
    drive, err := service.Drives.Get(cfg.SharedDriveID).Do()
    if err != nil {
        return nil, fmt.Errorf("failed to access shared drive: %v", err)
    }
    logger.Info("Connected to Shared Drive: %s", drive.Name)

    // Verify folder access if specified
    if cfg.FolderID != "" {
        folder, err := service.Files.Get(cfg.FolderID).
            SupportsAllDrives(true).
            Fields("id, name, parents").
            Do()
        if err != nil {
            return nil, fmt.Errorf("failed to access specified folder: %v", err)
        }

        var inSharedDrive bool
        for _, parent := range folder.Parents {
            if parent == cfg.SharedDriveID {
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

func (s *GoogleDriveService) ListAvailableBackups() ([]*DriveBackup, error) {
    query := "mimeType='application/zip' and trashed=false"

    var backups []*DriveBackup
    pageToken := ""

    for {
        fileList, err := s.service.Files.List().
            Q(query).
            OrderBy("createdTime desc").
            PageToken(pageToken).
            SupportsAllDrives(true).
            IncludeItemsFromAllDrives(true).
            Corpora("drive").
            DriveId(s.config.SharedDriveID).
            Fields("nextPageToken, files(id, name, createdTime, size, parents)").
            Do()

        if err != nil {
            return nil, fmt.Errorf("failed to list backup files: %v", err)
        }

        for _, file := range fileList.Files {
            createdTime, err := time.Parse(time.RFC3339, file.CreatedTime)
            if err != nil {
                s.logger.Warn("Failed to parse creation time for %s: %v", file.Name, err)
                continue
            }

            backups = append(backups, &DriveBackup{
                ID:          file.Id,
                Name:        file.Name,
                CreatedTime: createdTime,
                Size:        file.Size,
            })
            s.logger.Debug("Found backup: %s (Created: %s, Size: %s)",
                file.Name,
                createdTime.Format(time.RFC3339),
                formatBytes(file.Size))
        }

        pageToken = fileList.NextPageToken
        if pageToken == "" {
            break
        }
    }

    if len(backups) == 0 {
        // List all files for debugging
        allFiles, err := s.service.Files.List().
            SupportsAllDrives(true).
            IncludeItemsFromAllDrives(true).
            Corpora("drive").
            DriveId(s.config.SharedDriveID).
            Fields("files(id, name, mimeType, parents)").
            Do()
        if err != nil {
            s.logger.Error("Failed to list all files: %v", err)
        } else {
            s.logger.Info("Available files in drive:")
            for _, f := range allFiles.Files {
                s.logger.Info("- Name: %s, Type: %s, Parent: %v", f.Name, f.MimeType, f.Parents)
            }
        }
        return nil, fmt.Errorf("no backup files found in drive")
    }

    // Sort backups by time (newest first)
    sort.Slice(backups, func(i, j int) bool {
        return backups[i].CreatedTime.After(backups[j].CreatedTime)
    })

    return backups, nil
}

func (s *GoogleDriveService) GetLatestBackup(containerName string) (*DriveBackup, error) {
    query := fmt.Sprintf(
        "mimeType='application/zip' and name contains '%s' and name contains '.zip' and trashed=false",
        containerName,
    )

    s.logger.Debug("Searching for backups with query: %s", query)
    fileList, err := s.service.Files.List().
        Q(query).
        OrderBy("createdTime desc").
        PageSize(1).
        SupportsAllDrives(true).
        IncludeItemsFromAllDrives(true).
        Corpora("drive").
        DriveId(s.config.SharedDriveID).
        Fields("files(id, name, createdTime, size, parents)").
        Do()

    if err != nil {
        return nil, fmt.Errorf("failed to list backup files: %v", err)
    }

    if len(fileList.Files) == 0 {
        s.logger.Debug("No backups found. Checking available files...")
        s.ListAvailableBackups()
        return nil, fmt.Errorf("no backup files found for container: %s", containerName)
    }

    file := fileList.Files[0]
    createdTime, err := time.Parse(time.RFC3339, file.CreatedTime)
    if err != nil {
        return nil, fmt.Errorf("failed to parse creation time: %v", err)
    }

    s.logger.Info("Found latest backup: %s (Created: %s, Size: %s)",
        file.Name,
        createdTime.Format(time.RFC3339),
        formatBytes(file.Size))

    return &DriveBackup{
        ID:          file.Id,
        Name:        file.Name,
        CreatedTime: createdTime,
        Size:        file.Size,
    }, nil
}

func (s *GoogleDriveService) GetBackupFromDate(date time.Time, containerName string) (*DriveBackup, error) {
    startDate := date.Format("2006-01-02") + "T00:00:00Z"
    endDate := date.Add(24*time.Hour).Format("2006-01-02") + "T00:00:00Z"

    query := fmt.Sprintf(
        "mimeType='application/zip' and name contains '%s' and name contains '.zip' "+
            "and createdTime >= '%s' and createdTime < '%s' and trashed=false",
        containerName, startDate, endDate,
    )

    s.logger.Debug("Searching for backups with query: %s", query)
    fileList, err := s.service.Files.List().
        Q(query).
        OrderBy("createdTime desc").
        SupportsAllDrives(true).
        IncludeItemsFromAllDrives(true).
        Corpora("drive").
        DriveId(s.config.SharedDriveID).
        Fields("files(id, name, createdTime, size)").
        Do()

    if err != nil {
        return nil, fmt.Errorf("failed to list backup files: %v", err)
    }

    if len(fileList.Files) == 0 {
        s.logger.Debug("No backups found. Checking available files...")
        s.ListAvailableBackups()
        return nil, fmt.Errorf("no backup found for container %s on date %s",
            containerName, date.Format("2006-01-02"))
    }

    file := fileList.Files[0]
    createdTime, err := time.Parse(time.RFC3339, file.CreatedTime)
    if err != nil {
        return nil, fmt.Errorf("failed to parse creation time: %v", err)
    }

    s.logger.Info("Found backup from date %s: %s (Size: %s)",
        date.Format("2006-01-02"),
        file.Name,
        formatBytes(file.Size))

    return &DriveBackup{
        ID:          file.Id,
        Name:        file.Name,
        CreatedTime: createdTime,
        Size:        file.Size,
    }, nil
}

func (s *GoogleDriveService) DownloadFile(ctx context.Context, fileID string, destinationPath string) error {
    startTime := time.Now()
    s.logger.Info("Starting download of file: %s", fileID)

    res, err := s.service.Files.Get(fileID).
        SupportsAllDrives(true).
        Download()
    if err != nil {
        return fmt.Errorf("failed to download file: %v", err)
    }
    defer res.Body.Close()

    tempPath := destinationPath + ".tmp"
    out, err := os.Create(tempPath)
    if err != nil {
        return fmt.Errorf("failed to create temp file: %v", err)
    }

    written, err := io.Copy(out, res.Body)
    out.Close()

    if err != nil {
        os.Remove(tempPath)
        return fmt.Errorf("failed to save file: %v", err)
    }

    // Atomic rename
    if err := os.Rename(tempPath, destinationPath); err != nil {
        os.Remove(tempPath)
        return fmt.Errorf("failed to rename temp file: %v", err)
    }

    duration := time.Since(startTime)
    speed := float64(written) / duration.Seconds() / 1024 / 1024 // MB/s
    s.logger.Info("Download completed: %s (%.2f MB/s)", formatBytes(written), speed)

    return nil
}

// ProgressReader wraps an io.Reader to provide progress updates
type ProgressReader struct {
    io.Reader
    total     int64
    uploaded  int64
    onProgress func(uploaded, total int64)
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
    n, err := pr.Reader.Read(p)
    pr.uploaded += int64(n)
    if pr.onProgress != nil {
        pr.onProgress(pr.uploaded, pr.total)
    }
    return n, err
}

func (s *GoogleDriveService) UploadBackup(ctx context.Context, zipPath string, containerName string) error {
    // Create folder name with timestamp
    folderName := fmt.Sprintf("backup_%s_%s", containerName, time.Now().Format("20060102_150405"))

    // Create folder in Drive
    folder := &drive.File{
        Name:     folderName,
        MimeType: "application/vnd.google-apps.folder",
    }

    if s.config.SharedDriveID != "" {
        folder.Parents = []string{s.config.SharedDriveID}
        if s.config.FolderID != "" {
            folder.Parents = []string{s.config.FolderID}
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

    // Create progress reader
    progressReader := &ProgressReader{
        Reader: file,
        total:  fileInfo.Size(),
        onProgress: func(uploaded, total int64) {
            if uploaded == total {
                return // Skip 100% progress as we'll show completion message
            }
            percent := float64(uploaded) / float64(total) * 100
            elapsed := time.Since(startTime)
            speed := float64(uploaded) / elapsed.Seconds() / 1024 / 1024 // MB/s
            s.logger.Info("Upload progress: %.1f%% (%.2f MB/s)", percent, speed)
        },
    }

    result, err := s.service.Files.Create(zipFile).
        Media(progressReader).
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

    fileList, err := s.service.Files.List().
        Q(query).
        SupportsAllDrives(true).
        IncludeItemsFromAllDrives(true).
        Corpora("drive").
        DriveId(s.config.SharedDriveID).
        Fields("files(id, name, createdTime)").
        Do()

    if err != nil {
        return fmt.Errorf("failed to list old backups: %v", err)
    }

    for _, file := range fileList.Files {
        err := s.service.Files.Delete(file.Id).
            SupportsAllDrives(true).
            Do()

        if err != nil {
            s.logger.Error("Failed to delete old backup %s: %v", file.Name, err)
            continue
        }
        s.logger.Info("Deleted old backup: %s", file.Name)
    }

    return nil
}

func (s *GoogleDriveService) ListAvailableFolders() error {
    query := fmt.Sprintf("mimeType='application/vnd.google-apps.folder' and '%s' in parents and trashed=false",
        s.config.SharedDriveID)

    fileList, err := s.service.Files.List().
        Q(query).
        SupportsAllDrives(true).
        IncludeItemsFromAllDrives(true).
        Corpora("drive").
        DriveId(s.config.SharedDriveID).
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
