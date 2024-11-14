package restore

import (
    "context"
    "fmt"
    "time"
    "google.golang.org/api/drive/v3"
    "backup-service/internal/config"
    "backup-service/internal/utils"
)

func (s *GoogleDriveService) GetLatestBackup(containerName string) (*DriveBackup, error) {
    parentID := s.config.GoogleDrive.SharedDriveID
    if s.config.GoogleDrive.FolderID != "" {
        parentID = s.config.GoogleDrive.FolderID
    }

    // Search for backup files in the specified location
    query := fmt.Sprintf(
        "mimeType='application/zip' and name contains '%s_backup_' and '%s' in parents and trashed=false",
        containerName, parentID,
    )

    fileList, err := s.service.Files.List().
        Q(query).
        OrderBy("createdTime desc").
        PageSize(1).
        SupportsAllDrives(true).
        IncludeItemsFromAllDrives(true).
        Corpora("drive").
        DriveId(s.config.GoogleDrive.SharedDriveID).
        Fields("files(id, name, createdTime, size)").
        Do()

    if err != nil {
        return nil, fmt.Errorf("failed to list backup files: %v", err)
    }

    if len(fileList.Files) == 0 {
        return nil, fmt.Errorf("no backup files found")
    }

    file := fileList.Files[0]
    createdTime, err := time.Parse(time.RFC3339, file.CreatedTime)
    if err != nil {
        return nil, fmt.Errorf("failed to parse creation time: %v", err)
    }

    return &DriveBackup{
        ID:          file.Id,
        Name:        file.Name,
        CreatedTime: createdTime,
        Size:        file.Size,
    }, nil
}

func (s *GoogleDriveService) GetBackupFromDate(date time.Time, containerName string) (*DriveBackup, error) {
    parentID := s.config.GoogleDrive.SharedDriveID
    if s.config.GoogleDrive.FolderID != "" {
        parentID = s.config.GoogleDrive.FolderID
    }

    startDate := date.Format("2006-01-02") + "T00:00:00"
    endDate := date.Add(24*time.Hour).Format("2006-01-02") + "T00:00:00"

    query := fmt.Sprintf(
        "mimeType='application/zip' and name contains '%s_backup_' and '%s' in parents "+
        "and createdTime >= '%s' and createdTime < '%s' and trashed=false",
        containerName, parentID, startDate, endDate,
    )

    fileList, err := s.service.Files.List().
        Q(query).
        OrderBy("createdTime desc").
        SupportsAllDrives(true).
        IncludeItemsFromAllDrives(true).
        Corpora("drive").
        DriveId(s.config.GoogleDrive.SharedDriveID).
        Fields("files(id, name, createdTime, size)").
        Do()

    if err != nil {
        return nil, fmt.Errorf("failed to list backup files: %v", err)
    }

    if len(fileList.Files) == 0 {
        return nil, fmt.Errorf("no backup found for date: %s", date.Format("2006-01-02"))
    }

    file := fileList.Files[0]
    createdTime, err := time.Parse(time.RFC3339, file.CreatedTime)
    if err != nil {
        return nil, fmt.Errorf("failed to parse creation time: %v", err)
    }

    return &DriveBackup{
        ID:          file.Id,
        Name:        file.Name,
        CreatedTime: createdTime,
        Size:        file.Size,
    }, nil
}

func (s *GoogleDriveService) ListAvailableBackups() ([]*DriveBackup, error) {
    parentID := s.config.GoogleDrive.SharedDriveID
    if s.config.GoogleDrive.FolderID != "" {
        parentID = s.config.GoogleDrive.FolderID
        // Verify folder exists and is in the correct shared drive
        folder, err := s.service.Files.Get(parentID).
            SupportsAllDrives(true).
            Fields("id, name, parents").
            Do()
        if err != nil {
            return nil, fmt.Errorf("failed to access specified folder: %v", err)
        }

        var inSharedDrive bool
        for _, parent := range folder.Parents {
            if parent == s.config.GoogleDrive.SharedDriveID {
                inSharedDrive = true
                break
            }
        }
        if !inSharedDrive {
            return nil, fmt.Errorf("specified folder is not in the configured shared drive")
        }
        s.logger.Info("Using folder: %s", folder.Name)
    }

    query := fmt.Sprintf(
        "mimeType='application/zip' and name contains 'backup_' and '%s' in parents and trashed=false",
        parentID,
    )

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
            DriveId(s.config.GoogleDrive.SharedDriveID).
            Fields("nextPageToken, files(id, name, createdTime, size)").
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
        }

        pageToken = fileList.NextPageToken
        if pageToken == "" {
            break
        }
    }

    if len(backups) == 0 {
        return nil, fmt.Errorf("no backup files found")
    }

    return backups, nil
}