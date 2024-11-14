package restore

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "sort"
    "strings"
    "time"

    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
    "google.golang.org/api/drive/v3"
    "google.golang.org/api/option"

    "restore-service/internal/config"
    "restore-service/internal/utils"
)

type DriveBackup struct {
    ID          string
    Name        string
    CreatedTime time.Time
    Size        int64
}

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

    return &GoogleDriveService{
        service: service,
        config:  cfg,
        logger:  logger,
    }, nil
}

func (s *GoogleDriveService) GetLatestBackup() (*DriveBackup, error) {
    query := "name contains 'backup_' and mimeType='application/zip' and trashed=false"

    files, err := s.listBackups(query)
    if err != nil {
        return nil, err
    }

    if len(files) == 0 {
        return nil, fmt.Errorf("no backup files found")
    }

    // Sort by creation time descending
    sort.Slice(files, func(i, j int) bool {
        return files[i].CreatedTime.After(files[j].CreatedTime)
    })

    return files[0], nil
}

func (s *GoogleDriveService) GetBackupFromDate(date time.Time) (*DriveBackup, error) {
    // Format date for query
    startDate := date.Format("2006-01-02")
    endDate := date.Add(24 * time.Hour).Format("2006-01-02")

    query := fmt.Sprintf(
        "name contains 'backup_' and mimeType='application/zip' and "+
        "createdTime >= '%sT00:00:00' and createdTime < '%sT00:00:00' and "+
        "trashed=false",
        startDate, endDate)

    files, err := s.listBackups(query)
    if err != nil {
        return nil, err
    }

    if len(files) == 0 {
        return nil, fmt.Errorf("no backup found for date: %s", startDate)
    }

    // Return the latest backup from that date
    sort.Slice(files, func(i, j int) bool {
        return files[i].CreatedTime.After(files[j].CreatedTime)
    })

    return files[0], nil
}

func (s *GoogleDriveService) listBackups(query string) ([]*DriveBackup, error) {
    var backups []*DriveBackup
    pageToken := ""

    for {
        fileList, err := s.service.Files.List().
            Q(query).
            SupportsAllDrives(true).
            IncludeItemsFromAllDrives(true).
            Corpora("drive").
            DriveId(s.config.GoogleDrive.SharedDriveID).
            Fields("files(id, name, createdTime, size)").
            PageToken(pageToken).
            Do()

        if err != nil {
            return nil, fmt.Errorf("failed to list files: %v", err)
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
        }

        pageToken = fileList.NextPageToken
        if pageToken == "" {
            break
        }
    }

    return backups, nil
}

func (s *GoogleDriveService) DownloadFile(ctx context.Context, fileID string, destPath string) error {
    res, err := s.service.Files.Get(fileID).
        SupportsAllDrives(true).
        Download()
    if err != nil {
        return fmt.Errorf("failed to get file: %v", err)
    }
    defer res.Body.Close()

    out, err := os.Create(destPath)
    if err != nil {
        return fmt.Errorf("failed to create destination file: %v", err)
    }
    defer out.Close()

    startTime := time.Now()
    written, err := io.Copy(out, res.Body)
    if err != nil {
        return fmt.Errorf("failed to save file: %v", err)
    }

    duration := time.Since(startTime)
    speed := float64(written) / 1024 / 1024 / duration.Seconds() // MB/s
    s.logger.Info("Download completed: %d bytes (%.2f MB/s)", written, speed)

    return nil
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