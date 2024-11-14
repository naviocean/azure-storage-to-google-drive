package config

import (
    "fmt"
    "os"
    "strconv"
)

type AzureConfig struct {
    AccountName  string
    AccountKey   string
    ContainerName string
}

type GoogleDriveConfig struct {
    CredentialsPath string
    TokenPath       string
    SharedDriveID   string
}

type RestoreConfig struct {
    TempDir        string
    MaxConcurrent  int
}

type Config struct {
    Azure       AzureConfig
    GoogleDrive GoogleDriveConfig
    Restore     RestoreConfig
    LogLevel    string
}

func LoadConfig() (*Config, error) {
    config := &Config{
        Azure: AzureConfig{
            AccountName:   os.Getenv("TARGET_AZURE_ACCOUNT_NAME"),
            AccountKey:    os.Getenv("TARGET_AZURE_ACCOUNT_KEY"),
            ContainerName: os.Getenv("TARGET_AZURE_CONTAINER_NAME"),
        },
        GoogleDrive: GoogleDriveConfig{
            CredentialsPath: getEnvWithDefault("GOOGLE_CREDENTIALS_PATH", "/app/credentials.json"),
            TokenPath:       getEnvWithDefault("GOOGLE_TOKEN_PATH", "/app/token.json"),
            SharedDriveID:   os.Getenv("GOOGLE_SHARED_DRIVE_ID"),
        },
        Restore: RestoreConfig{
            TempDir:       getEnvWithDefault("TEMP_DIR", "/app/temp"),
            MaxConcurrent: getEnvAsIntWithDefault("MAX_CONCURRENT_OPERATIONS", 10),
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
    if cfg.Azure.AccountName == "" || cfg.Azure.AccountKey == "" || cfg.Azure.ContainerName == "" {
        return fmt.Errorf("target azure configuration is incomplete")
    }

    // Validate Google Drive config
    if cfg.GoogleDrive.SharedDriveID == "" {
        return fmt.Errorf("shared drive ID is required")
    }

    // Create temp directory if not exists
    if err := os.MkdirAll(cfg.Restore.TempDir, 0755); err != nil {
        return fmt.Errorf("failed to create temp directory: %v", err)
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