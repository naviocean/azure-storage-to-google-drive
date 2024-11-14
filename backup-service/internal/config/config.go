package config

import (
    "fmt"
    "os"
    "strconv"
    "time"
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
    FolderID        string
}

type BackupConfig struct {
    Schedule       string
    RetentionDays  int
    MaxConcurrent  int
    BackupPath     string
    TempDir        string
    TimeZone       *time.Location
}

type Config struct {
    Azure       AzureConfig
    GoogleDrive GoogleDriveConfig
    Backup      BackupConfig
    LogLevel    string
}

func LoadConfig() (*Config, error) {
    // Load timezone
    tz := getEnvWithDefault("TZ", "Asia/Ho_Chi_Minh")
    location, err := time.LoadLocation(tz)
    if err != nil {
        return nil, fmt.Errorf("invalid timezone: %v", err)
    }

    config := &Config{
        Azure: AzureConfig{
            AccountName:   os.Getenv("AZURE_ACCOUNT_NAME"),
            AccountKey:    os.Getenv("AZURE_ACCOUNT_KEY"),
            ContainerName: getEnvWithDefault("AZURE_CONTAINER_NAME", "ALL"),
        },
        GoogleDrive: GoogleDriveConfig{
            CredentialsPath: getEnvWithDefault("GOOGLE_CREDENTIALS_PATH", "/app/credentials.json"),
            TokenPath:       getEnvWithDefault("GOOGLE_TOKEN_PATH", "/app/token.json"),
            SharedDriveID:   os.Getenv("GOOGLE_SHARED_DRIVE_ID"),
            FolderID:        os.Getenv("GOOGLE_FOLDER_ID"), // Optional: if not set, will use root of Shared Drive
        },
        Backup: BackupConfig{
            Schedule:      getEnvWithDefault("BACKUP_SCHEDULE", "0 1 * * *"),
            RetentionDays: getEnvAsIntWithDefault("BACKUP_RETENTION_DAYS", 7),
            MaxConcurrent: getEnvAsIntWithDefault("MAX_CONCURRENT_OPERATIONS", 10),
            BackupPath:    getEnvWithDefault("BACKUP_PATH", "/app/backups"),
            TempDir:       getEnvWithDefault("TEMP_DIR", "/app/temp"),
            TimeZone:      location,
        },
        LogLevel: getEnvWithDefault("LOG_LEVEL", "info"),
    }

    if err := validateConfig(config); err != nil {
        return nil, err
    }

    return config, nil
}

func validateConfig(cfg *Config) error {
    // Validate Azure config
    if cfg.Azure.AccountName == "" || cfg.Azure.AccountKey == "" {
        return fmt.Errorf("azure storage account configuration is incomplete")
    }

    // Validate Google Drive config
    if cfg.GoogleDrive.SharedDriveID == "" {
        return fmt.Errorf("google drive shared drive ID is required")
    }

    // Create necessary directories
    dirs := []string{cfg.Backup.BackupPath, cfg.Backup.TempDir}
    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("failed to create directory %s: %v", dir, err)
        }
    }

    return nil
}

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