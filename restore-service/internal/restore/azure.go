package restore

import (
    "context"
    "fmt"
    "net/url"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"

    "github.com/Azure/azure-storage-blob-go/azblob"
    "shared/pkg/config"
    "shared/pkg/utils"
)

type UploadStats struct {
    FilesCount int
    TotalSize  int64
    Errors     []error
}

type AzureService struct {
    serviceURL azblob.ServiceURL
    config    *config.RestoreServiceConfig
    logger    *utils.Logger
}

func NewAzureService(cfg *config.RestoreServiceConfig, logger *utils.Logger) (*AzureService, error) {
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
        serviceURL: serviceURL,
        config:    cfg,
        logger:    logger,
    }, nil
}

func (s *AzureService) UploadFiles(ctx context.Context, sourcePath string, containerName string) (*UploadStats, error) {
    stats := &UploadStats{}
    var mu sync.Mutex
    var wg sync.WaitGroup
    maxConcurrent := 10
    semaphore := make(chan struct{}, maxConcurrent)
    errChan := make(chan error, 100)

    // Create container if not exists
    containerURL := s.serviceURL.NewContainerURL(containerName)
    _, err := containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessNone)
    if err != nil && !strings.Contains(err.Error(), "ContainerAlreadyExists") {
        return stats, fmt.Errorf("failed to create container: %v", err)
    }

    err = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if info.IsDir() {
            return nil
        }

        relPath, err := filepath.Rel(sourcePath, path)
        if err != nil {
            return fmt.Errorf("failed to get relative path: %v", err)
        }

        wg.Add(1)
        go func() {
            defer wg.Done()
            semaphore <- struct{}{}
            defer func() { <-semaphore }()

            if err := s.uploadFile(ctx, containerURL, path, relPath); err != nil {
                errChan <- fmt.Errorf("failed to upload %s: %v", relPath, err)
                return
            }

            mu.Lock()
            stats.FilesCount++
            stats.TotalSize += info.Size()
            mu.Unlock()

            s.logger.Info("Uploaded: %s", relPath)
        }()

        return nil
    })

    wg.Wait()
    close(errChan)

    for err := range errChan {
        stats.Errors = append(stats.Errors, err)
    }

    if err != nil {
        return stats, fmt.Errorf("failed to walk source directory: %v", err)
    }

    if len(stats.Errors) > 0 {
        return stats, fmt.Errorf("encountered %d upload errors", len(stats.Errors))
    }

    return stats, nil
}

func (s *AzureService) uploadFile(ctx context.Context, containerURL azblob.ContainerURL, sourcePath, blobName string) error {
    blobURL := containerURL.NewBlockBlobURL(blobName)

    file, err := os.Open(sourcePath)
    if err != nil {
        return fmt.Errorf("failed to open source file: %v", err)
    }
    defer file.Close()

    _, err = blobURL.Upload(ctx,
        file,
        azblob.BlobHTTPHeaders{},
        azblob.Metadata{},
        azblob.BlobAccessConditions{},
        azblob.DefaultAccessTier,
        azblob.BlobTagsMap{},
        azblob.ClientProvidedKeyOptions{},
        azblob.ImmutabilityPolicyOptions{},
    )

    if err != nil {
        return fmt.Errorf("failed to upload blob: %v", err)
    }

    return nil
}