package config

import (
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "time"

    "github.com/robfig/cron/v3"
)

type AzureConfig struct {
    AccountName   string
    AccountKey    string
    ContainerName string  // "ALL" hoặc tên container cụ thể
}

type GoogleDriveConfig struct {
    CredentialsPath string
    TokenPath       string
    SharedDriveID   string
    FolderID        string  // Optional: ID của folder trong Shared Drive
}

type BackupConfig struct {
    Schedule       string
    RetentionDays  int
    MaxConcurrent  int
    BackupPath     string
    TempDir        string
    TimeZone       *time.Location
}

// Cấu hình chung
type CommonConfig struct {
    LogLevel      string
    EnableMetrics bool
    MetricsPort   int
}

// Config cho backup service
type BackupServiceConfig struct {
    Azure       AzureConfig
    GoogleDrive GoogleDriveConfig
    Backup      BackupConfig
    Common      CommonConfig
}

// Config cho restore service
type RestoreServiceConfig struct {
    Azure       AzureConfig        // Target Azure Storage
    GoogleDrive GoogleDriveConfig
    TempDir     string
    Common      CommonConfig
}

// LoadBackupConfig loads configuration for backup service
func LoadBackupConfig() (*BackupServiceConfig, error) {
    // Load timezone
    tz := getEnvWithDefault("TZ", "Asia/Ho_Chi_Minh")
    location, err := time.LoadLocation(tz)
    if err != nil {
        return nil, fmt.Errorf("invalid timezone: %v", err)
    }

    config := &BackupServiceConfig{
        Azure: AzureConfig{
            AccountName:   os.Getenv("AZURE_ACCOUNT_NAME"),
            AccountKey:    os.Getenv("AZURE_ACCOUNT_KEY"),
            ContainerName: getEnvWithDefault("AZURE_CONTAINER_NAME", "ALL"),
        },
        GoogleDrive: GoogleDriveConfig{
            CredentialsPath: getEnvWithDefault("GOOGLE_CREDENTIALS_PATH", "/app/credentials.json"),
            TokenPath:       getEnvWithDefault("GOOGLE_TOKEN_PATH", "/app/token.json"),
            SharedDriveID:   os.Getenv("GOOGLE_SHARED_DRIVE_ID"),
            FolderID:        os.Getenv("GOOGLE_FOLDER_ID"),
        },
        Backup: BackupConfig{
            Schedule:      getEnvWithDefault("BACKUP_SCHEDULE", "0 1 * * *"),
            RetentionDays: getEnvAsIntWithDefault("BACKUP_RETENTION_DAYS", 7),
            MaxConcurrent: getEnvAsIntWithDefault("MAX_CONCURRENT_OPERATIONS", 10),
            BackupPath:    getEnvWithDefault("BACKUP_PATH", "/app/backups"),
            TempDir:       getEnvWithDefault("TEMP_DIR", "/app/temp"),
            TimeZone:      location,
        },
        Common: CommonConfig{
            LogLevel:      getEnvWithDefault("LOG_LEVEL", "info"),
            EnableMetrics: getEnvAsBoolWithDefault("ENABLE_METRICS", true),
            MetricsPort:   getEnvAsIntWithDefault("METRICS_PORT", 9090),
        },
    }

    if err := validateBackupConfig(config); err != nil {
        return nil, err
    }

    return config, nil
}

func LoadRestoreConfig() (*RestoreServiceConfig, error) {
    config := &RestoreServiceConfig{
        Azure: AzureConfig{
            AccountName:   os.Getenv("TARGET_AZURE_ACCOUNT_NAME"),
            AccountKey:    os.Getenv("TARGET_AZURE_ACCOUNT_KEY"),
            ContainerName: getEnvWithDefault("TARGET_AZURE_CONTAINER_NAME", "ALL"),
        },
        GoogleDrive: GoogleDriveConfig{
            CredentialsPath: getEnvWithDefault("GOOGLE_CREDENTIALS_PATH", "/app/credentials.json"),
            TokenPath:       getEnvWithDefault("GOOGLE_TOKEN_PATH", "/app/token.json"),
            SharedDriveID:   os.Getenv("GOOGLE_SHARED_DRIVE_ID"),
            FolderID:        os.Getenv("GOOGLE_FOLDER_ID"),
        },
        TempDir: getEnvWithDefault("TEMP_DIR", "/app/temp"),
        Common: CommonConfig{
            LogLevel:      getEnvWithDefault("LOG_LEVEL", "info"),
            EnableMetrics: getEnvAsBoolWithDefault("ENABLE_METRICS", true),
            MetricsPort:   getEnvAsIntWithDefault("METRICS_PORT", 9090),
        },
    }

    if err := validateRestoreConfig(config); err != nil {
        return nil, err
    }

    return config, nil
}

func validateBackupConfig(cfg *BackupServiceConfig) error {
    // Validate Azure config
    if cfg.Azure.AccountName == "" || cfg.Azure.AccountKey == "" {
        return fmt.Errorf("azure storage account configuration is incomplete")
    }

    // Validate Google Drive config
    if cfg.GoogleDrive.SharedDriveID == "" {
        return fmt.Errorf("google shared drive ID is required")
    }

    // Validate paths
    paths := []string{
        cfg.Backup.BackupPath,
        cfg.Backup.TempDir,
        filepath.Dir(cfg.GoogleDrive.CredentialsPath),
        filepath.Dir(cfg.GoogleDrive.TokenPath),
    }

    for _, path := range paths {
        if err := os.MkdirAll(path, 0755); err != nil {
            return fmt.Errorf("failed to create directory %s: %v", path, err)
        }
    }

    // Validate schedule format
    if _, err := cron.ParseStandard(cfg.Backup.Schedule); err != nil {
        return fmt.Errorf("invalid backup schedule: %v", err)
    }

    return nil
}

func validateRestoreConfig(cfg *RestoreServiceConfig) error {
    // Validate Target Azure config
    if cfg.Azure.AccountName == "" || cfg.Azure.AccountKey == "" {
        return fmt.Errorf("target azure storage account configuration is incomplete")
    }

    // Validate Google Drive config
    if cfg.GoogleDrive.SharedDriveID == "" {
        return fmt.Errorf("google shared drive ID is required")
    }

    // Validate paths
    paths := []string{
        cfg.TempDir,
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

// Helper functions
func getEnvWithDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvAsIntWithDefault(key string, defaultValue int) int {
    strValue := os.Getenv(key)
    if strValue == "" {
        return defaultValue
    }

    value, err := strconv.Atoi(strValue)
    if err != nil {
        return defaultValue
    }
    return value
}

func getEnvAsBoolWithDefault(key string, defaultValue bool) bool {
    strValue := os.Getenv(key)
    if strValue == "" {
        return defaultValue
    }

    value, err := strconv.ParseBool(strValue)
    if err != nil {
        return defaultValue
    }
    return value
}