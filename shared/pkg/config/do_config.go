package config

import (
    "fmt"
    "os"
    "path/filepath"
    "time"
)

type SpacesConfig struct {
    Endpoint        string
    Region          string
    AccessKeyID     string
    SecretAccessKey string
    BucketName      string
}

type DORestoreConfig struct {
    TempDir       string
    ContainerName string
}

type DORestoreServiceConfig struct {
    Azure       AzureConfig
    GoogleDrive GoogleDriveConfig
    Spaces      SpacesConfig
    Restore     DORestoreConfig
    TimeZone    *time.Location
    Common      CommonConfig
}

func LoadDORestoreConfig() (*DORestoreServiceConfig, error) {
    // Load timezone
    tz := getEnvWithDefault("TZ", "Asia/Ho_Chi_Minh")
    location, err := time.LoadLocation(tz)
    if err != nil {
        return nil, fmt.Errorf("invalid timezone: %v", err)
    }

    config := &DORestoreServiceConfig{
        Common: CommonConfig{
            LogLevel:      getEnvWithDefault("LOG_LEVEL", "info"),
            EnableMetrics: getEnvAsBoolWithDefault("ENABLE_METRICS", true),
            MetricsPort:   getEnvAsIntWithDefault("METRICS_PORT", 9090),
        },
        GoogleDrive: GoogleDriveConfig{
            CredentialsPath: getEnvWithDefault("GOOGLE_CREDENTIALS_PATH", "/app/credentials.json"),
            TokenPath:       getEnvWithDefault("GOOGLE_TOKEN_PATH", "/app/token.json"),
            SharedDriveID:   os.Getenv("GOOGLE_SHARED_DRIVE_ID"),
            FolderID:        os.Getenv("GOOGLE_FOLDER_ID"),
        },
        Spaces: SpacesConfig{
            Endpoint:        getEnvWithDefault("SPACES_ENDPOINT", "https://sgp1.digitaloceanspaces.com"),
            Region:         getEnvWithDefault("SPACES_REGION", "sgp1"),
            AccessKeyID:     os.Getenv("SPACES_ACCESS_KEY_ID"),
            SecretAccessKey: os.Getenv("SPACES_SECRET_ACCESS_KEY"),
            BucketName:     os.Getenv("SPACES_BUCKET_NAME"),
        },
        Restore: DORestoreConfig{
            TempDir:       getEnvWithDefault("TEMP_DIR", "/app/temp"),
            ContainerName: os.Getenv("RESTORE_CONTAINER_NAME"),
        },
        TimeZone: location,
    }

    if err := validateDORestoreConfig(config); err != nil {
        return nil, err
    }

    return config, nil
}

func validateDORestoreConfig(cfg *DORestoreServiceConfig) error {
    // Validate Google Drive config
    if cfg.GoogleDrive.SharedDriveID == "" {
        return fmt.Errorf("google shared drive ID is required")
    }

    // Validate Spaces config
    if cfg.Spaces.AccessKeyID == "" || cfg.Spaces.SecretAccessKey == "" {
        return fmt.Errorf("spaces credentials are required")
    }
    if cfg.Spaces.BucketName == "" {
        return fmt.Errorf("spaces bucket name is required")
    }

    // Validate Restore config
    if cfg.Restore.ContainerName == "" {
        return fmt.Errorf("restore container name is required")
    }

    // Validate paths
    paths := []string{
        cfg.Restore.TempDir,
        filepath.Dir(cfg.GoogleDrive.CredentialsPath),
        filepath.Dir(cfg.GoogleDrive.TokenPath),
    }

    for _, path := range paths {
        if err := os.MkdirAll(path, 0755); err != nil {
            return fmt.Errorf("failed to create directory %s: %v", path, err)
        }
    }

    return nil
}