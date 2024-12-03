package spaces

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/s3/types"

    sconfig "shared/pkg/config"
    "shared/pkg/utils"
)

type UploadStats struct {
    FilesCount int64
    TotalSize  int64
}

type SpacesService struct {
    client *s3.Client
    config *sconfig.DORestoreServiceConfig
    logger *utils.Logger
}

func NewSpacesService(cfg *sconfig.DORestoreServiceConfig, logger *utils.Logger) (*SpacesService, error) {
    resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
        return aws.Endpoint{
            URL: cfg.Spaces.Endpoint,
        }, nil
    })

    customProvider := credentials.NewStaticCredentialsProvider(
        cfg.Spaces.AccessKeyID,
        cfg.Spaces.SecretAccessKey,
        "",
    )

    awsCfg, err := config.LoadDefaultConfig(context.Background(),
        config.WithEndpointResolverWithOptions(resolver),
        config.WithCredentialsProvider(customProvider),
        config.WithRegion(cfg.Spaces.Region),
    )
    if err != nil {
        return nil, fmt.Errorf("unable to load AWS SDK config: %v", err)
    }

    client := s3.NewFromConfig(awsCfg)

    // Verify bucket access
    _, err = client.HeadBucket(context.Background(), &s3.HeadBucketInput{
        Bucket: aws.String(cfg.Spaces.BucketName),
    })
    if err != nil {
        return nil, fmt.Errorf("failed to access bucket: %v", err)
    }

    logger.Info("Connected to Spaces bucket: %s", cfg.Spaces.BucketName)

    return &SpacesService{
        client: client,
        config: cfg,
        logger: logger,
    }, nil
}

func (s *SpacesService) UploadFiles(ctx context.Context, sourcePath string, prefix string) (*UploadStats, error) {
    stats := &UploadStats{}

    err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if info.IsDir() {
            return nil
        }

        // Calculate object key (path in the bucket)
        relPath, err := filepath.Rel(sourcePath, path)
        if err != nil {
            return fmt.Errorf("failed to get relative path: %v", err)
        }

        // Convert Windows path to Unix style
        relPath = filepath.ToSlash(relPath)
        objectKey := filepath.Join(prefix, relPath)

        // Open file
        file, err := os.Open(path)
        if err != nil {
            return fmt.Errorf("failed to open file %s: %v", path, err)
        }
        defer file.Close()

        // Create progress reader
        progressReader := &utils.ProgressReader{
            Reader: file,
            Total:  info.Size(),
            OnProgress: func(uploaded, total int64) {
                if uploaded == total {
                    return // Skip 100% progress
                }
                percent := float64(uploaded) / float64(total) * 100
                s.logger.Info("Uploading %s: %.1f%%", relPath, percent)
            },
        }

        startTime := time.Now()
        s.logger.Info("Starting upload of %s (%s)", relPath, utils.FormatBytes(info.Size()))

        // Upload file
        _, err = s.client.PutObject(ctx, &s3.PutObjectInput{
            Bucket:        aws.String(s.config.Spaces.BucketName),
            Key:           aws.String(objectKey),
            Body:         progressReader,
            ContentLength: aws.Int64(info.Size()),
        })
        if err != nil {
            return fmt.Errorf("failed to upload %s: %v", path, err)
        }

        duration := time.Since(startTime)
        speed := float64(info.Size()) / duration.Seconds() / 1024 / 1024 // MB/s
        s.logger.Info("Uploaded %s (%s, %.2f MB/s)", relPath, utils.FormatBytes(info.Size()), speed)

        stats.FilesCount++
        stats.TotalSize += info.Size()

        return nil
    })

    if err != nil {
        return nil, fmt.Errorf("upload failed: %v", err)
    }

    return stats, nil
}

func (s *SpacesService) DeletePrefix(ctx context.Context, prefix string) error {
    // List all objects with the prefix
    var continuationToken *string
    for {
        input := &s3.ListObjectsV2Input{
            Bucket: aws.String(s.config.Spaces.BucketName),
            Prefix: aws.String(prefix),
        }
        if continuationToken != nil {
            input.ContinuationToken = continuationToken
        }

        output, err := s.client.ListObjectsV2(ctx, input)
        if err != nil {
            return fmt.Errorf("failed to list objects: %v", err)
        }

        // Delete objects in batches
        if len(output.Contents) > 0 {
            objects := make([]types.ObjectIdentifier, len(output.Contents))
            for i, obj := range output.Contents {
                objects[i] = types.ObjectIdentifier{
                    Key: obj.Key,
                }
            }

            _, err = s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
                Bucket: aws.String(s.config.Spaces.BucketName),
                Delete: &types.Delete{
                    Objects: objects,
                    Quiet:   aws.Bool(true),
                },
            })
            if err != nil {
                return fmt.Errorf("failed to delete objects: %v", err)
            }

            s.logger.Info("Deleted %d objects with prefix: %s", len(objects), prefix)
        }

        if !aws.ToBool(output.IsTruncated) {
            break
        }
        continuationToken = output.NextContinuationToken
    }

    return nil
}