package backup

import (
    "context"
    "crypto/md5"
    "encoding/json"
    "fmt"
    "io"
    "net/url"
    "os"
    "path/filepath"
    "sync"
    "time"

    "github.com/Azure/azure-storage-blob-go/azblob"
    "backup-service/internal/config"
    "backup-service/internal/utils"
)

type BlobMetadata struct {
    LastModified time.Time `json:"lastModified"`
    MD5Hash      string    `json:"md5hash"`
    Size         int64     `json:"size"`
}

type ContainerMetadata struct {
    Files    map[string]BlobMetadata `json:"files"`
    LastSync time.Time              `json:"lastSync"`
}

type SyncMetadata struct {
    LastSync   time.Time                         `json:"lastSync"`
    Containers map[string]ContainerMetadata      `json:"containers"`
}

type ContainerStats struct {
    FilesCount      int   `json:"filesCount"`
    TotalSize       int64 `json:"totalSize"`
    DownloadedFiles int   `json:"downloadedFiles"`
    SkippedFiles    int   `json:"skippedFiles"`
}

type AzureService struct {
    serviceURL    azblob.ServiceURL
    config       *config.Config
    logger       *utils.Logger
    metadataPath string
}

func NewAzureService(cfg *config.Config, logger *utils.Logger) (*AzureService, error) {
    credential, err := azblob.NewSharedKeyCredential(
        cfg.Azure.AccountName,
        cfg.Azure.AccountKey,
    )
    if err != nil {
        return nil, fmt.Errorf("invalid credentials: %v", err)
    }

    pipeline := azblob.NewPipeline(credential, azblob.PipelineOptions{
        Retry: azblob.RetryOptions{
            MaxTries:      3,
            TryTimeout:    2 * time.Minute,
            RetryDelay:    time.Second * 5,
            MaxRetryDelay: time.Second * 30,
        },
    })

    URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/",
        cfg.Azure.AccountName))
    serviceURL := azblob.NewServiceURL(*URL, pipeline)

    return &AzureService{
        serviceURL:    serviceURL,
        config:       cfg,
        logger:       logger,
        metadataPath: filepath.Join(cfg.Backup.BackupPath, "sync_metadata.json"),
    }, nil
}

func (s *AzureService) loadSyncMetadata() (*SyncMetadata, error) {
    metadata := &SyncMetadata{
        Containers: make(map[string]ContainerMetadata),
    }

    if _, err := os.Stat(s.metadataPath); os.IsNotExist(err) {
        return metadata, nil
    }

    file, err := os.Open(s.metadataPath)
    if err != nil {
        return metadata, err
    }
    defer file.Close()

    if err := json.NewDecoder(file).Decode(metadata); err != nil {
        return metadata, err
    }

    return metadata, nil
}

func (s *AzureService) saveSyncMetadata(metadata *SyncMetadata) error {
    // Create temp file for atomic write
    tempFile := s.metadataPath + ".tmp"
    file, err := os.Create(tempFile)
    if err != nil {
        return err
    }

    encoder := json.NewEncoder(file)
    encoder.SetIndent("", "    ")
    if err := encoder.Encode(metadata); err != nil {
        file.Close()
        os.Remove(tempFile)
        return err
    }

    if err := file.Close(); err != nil {
        os.Remove(tempFile)
        return err
    }

    // Atomic rename
    return os.Rename(tempFile, s.metadataPath)
}

func (s *AzureService) calculateMD5(filePath string) (string, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return "", err
    }
    defer file.Close()

    hash := md5.New()
    if _, err := io.Copy(hash, file); err != nil {
        return "", err
    }

    return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func (s *AzureService) DownloadBlobs(ctx context.Context, backupRootDir string) (map[string]*ContainerStats, error) {
    startTime := time.Now()
    s.logger.Info("Starting blob download to: %s", backupRootDir)

    metadata, err := s.loadSyncMetadata()
    if err != nil {
        s.logger.Warn("Failed to load sync metadata, will perform full sync: %v", err)
        metadata = &SyncMetadata{
            Containers: make(map[string]ContainerMetadata),
        }
    }

    stats := make(map[string]*ContainerStats)
    var mu sync.Mutex

    if s.config.Azure.ContainerName == "ALL" {
        // Process all containers
        var containerWg sync.WaitGroup
        containerSemaphore := make(chan struct{}, 5) // Limit concurrent containers

        for marker := (azblob.Marker{}); marker.NotDone(); {
            listContainer, err := s.serviceURL.ListContainersSegment(ctx, marker, azblob.ListContainersSegmentOptions{})
            if err != nil {
                return nil, fmt.Errorf("failed to list containers: %v", err)
            }

            marker = listContainer.NextMarker

            for _, container := range listContainer.ContainerItems {
                containerWg.Add(1)
                go func(container azblob.ContainerItem) {
                    defer containerWg.Done()
                    containerSemaphore <- struct{}{} // Acquire
                    defer func() { <-containerSemaphore }() // Release

                    s.logger.Info("Processing container: %s", container.Name)
                    containerStats, err := s.processContainer(
                        ctx,
                        container.Name,
                        backupRootDir,
                        metadata.Containers[container.Name],
                    )
                    if err != nil {
                        s.logger.Error("Failed to process container %s: %v", container.Name, err)
                        return
                    }

                    mu.Lock()
                    stats[container.Name] = containerStats
                    mu.Unlock()

                }(container)
            }

            containerWg.Wait()
        }

    } else {
        // Process single container
        containerStats, err := s.processContainer(
            ctx,
            s.config.Azure.ContainerName,
            backupRootDir,
            metadata.Containers[s.config.Azure.ContainerName],
        )
        if err != nil {
            return nil, fmt.Errorf("failed to process container %s: %v", s.config.Azure.ContainerName, err)
        }
        stats[s.config.Azure.ContainerName] = containerStats
    }

    // Update metadata
    newMetadata := &SyncMetadata{
        LastSync:   time.Now(),
        Containers: make(map[string]ContainerMetadata),
    }

    // Save updated metadata
    if err := s.saveSyncMetadata(newMetadata); err != nil {
        s.logger.Error("Failed to save sync metadata: %v", err)
    }

    duration := time.Since(startTime)
    var totalFiles, totalSize int64
    for _, containerStats := range stats {
        totalFiles += int64(containerStats.FilesCount)
        totalSize += containerStats.TotalSize
    }

    s.logger.Info("Sync completed in %v: processed %d containers, %d files, %.2f MB",
        duration,
        len(stats),
        totalFiles,
        float64(totalSize)/(1024*1024))

    return stats, nil
}

func (s *AzureService) processContainer(ctx context.Context, containerName string, backupRootDir string, metadata ContainerMetadata) (*ContainerStats, error) {
    containerURL := s.serviceURL.NewContainerURL(containerName)

    // Verify container exists and is accessible
    _, err := containerURL.GetProperties(ctx, azblob.LeaseAccessConditions{})
    if err != nil {
        return nil, fmt.Errorf("container not accessible: %v", err)
    }

    stats := &ContainerStats{}
    currentFiles := make(map[string]struct{}) // Track current files
    var mu sync.Mutex
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, s.config.Backup.MaxConcurrent)
    errChan := make(chan error, s.config.Backup.MaxConcurrent)

    // Create permanent container directory
    containerDir := filepath.Join(backupRootDir, containerName)
    if err := os.MkdirAll(containerDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create container directory: %v", err)
    }

    // List and process blobs
    for marker := (azblob.Marker{}); marker.NotDone(); {
        listBlob, err := containerURL.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{
            MaxResults: 5000,
        })
        if err != nil {
            return nil, fmt.Errorf("failed to list blobs: %v", err)
        }

        marker = listBlob.NextMarker

        for _, blobInfo := range listBlob.Segment.BlobItems {
            wg.Add(1)
            go func(blobInfo azblob.BlobItemInternal) {
                defer wg.Done()

                semaphore <- struct{}{} // Acquire
                defer func() { <-semaphore }() // Release

                mu.Lock()
                stats.FilesCount++
                if blobInfo.Properties.ContentLength != nil {
                    stats.TotalSize += *blobInfo.Properties.ContentLength
                }
                currentFiles[blobInfo.Name] = struct{}{} // Track current file
                mu.Unlock()

                // Check if blob needs download
                previousMetadata, exists := metadata.Files[blobInfo.Name]
                needsDownload := true

                if exists {
                    targetPath := filepath.Join(containerDir, blobInfo.Name)
                    if _, err := os.Stat(targetPath); err == nil { // File exists locally
                        if blobInfo.Properties.LastModified.Equal(previousMetadata.LastModified) {
                            mu.Lock()
                            stats.SkippedFiles++
                            mu.Unlock()
                            needsDownload = false
                            s.logger.Debug("[%s] File unchanged: %s", containerName, blobInfo.Name)
                        }
                    }
                }

                if needsDownload {
                    targetPath := filepath.Join(containerDir, blobInfo.Name)
                    if err := s.downloadBlob(ctx, containerURL, blobInfo.Name, targetPath); err != nil {
                        errChan <- fmt.Errorf("error downloading %s: %v", blobInfo.Name, err)
                        return
                    }

                    mu.Lock()
                    stats.DownloadedFiles++
                    mu.Unlock()

                    s.logger.Info("[%s] Downloaded: %s", containerName, blobInfo.Name)
                }
            }(blobInfo)
        }
    }

    wg.Wait()
    close(errChan)

    // Check for files that no longer exist in Azure
    err = filepath.Walk(containerDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() {
            relPath, err := filepath.Rel(containerDir, path)
            if err != nil {
                return err
            }
            if _, exists := currentFiles[relPath]; !exists {
                s.logger.Info("[%s] Removing deleted file: %s", containerName, relPath)
                if err := os.Remove(path); err != nil {
                    return err
                }
            }
        }
        return nil
    })

    if err != nil {
        s.logger.Error("[%s] Error cleaning up deleted files: %v", containerName, err)
    }

    // Check for download errors
    var errors []error
    for err := range errChan {
        errors = append(errors, err)
    }

    if len(errors) > 0 {
        return stats, fmt.Errorf("encountered %d download errors: %v", len(errors), errors)
    }

    return stats, nil
}

func (s *AzureService) downloadBlob(ctx context.Context, containerURL azblob.ContainerURL, blobName, targetPath string) error {
    blobURL := containerURL.NewBlockBlobURL(blobName)

    // Create parent directories if needed
    if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
        return fmt.Errorf("failed to create directory: %v", err)
    }

    // Create temp file
    tempPath := targetPath + ".tmp"
    outFile, err := os.Create(tempPath)
    if err != nil {
        return fmt.Errorf("failed to create temp file: %v", err)
    }
    defer outFile.Close()

    // Download to temp file
    downloadResponse, err := blobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
    if err != nil {
        os.Remove(tempPath)
        return fmt.Errorf("failed to download blob: %v", err)
    }

    reader := downloadResponse.Body(azblob.RetryReaderOptions{
        MaxRetryRequests: 3,
    })
    defer reader.Close()

    // Copy with progress tracking
    written, err := io.Copy(outFile, reader)
    if err != nil {
        os.Remove(tempPath)
        return fmt.Errorf("failed to save blob data: %v", err)
    }

    // Sync to disk
    if err := outFile.Sync(); err != nil {
        os.Remove(tempPath)
        return fmt.Errorf("failed to sync file: %v", err)
    }

    // Close file before rename
    outFile.Close()

    // Atomic rename
    if err := os.Rename(tempPath, targetPath); err != nil {
        os.Remove(tempPath)
        return fmt.Errorf("failed to rename temp file: %v", err)
    }

    s.logger.Debug("Downloaded %s (%d bytes)", blobName, written)
    return nil
}