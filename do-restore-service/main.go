package main

import (
    "context"
    "fmt"
    "os"

    "shared/pkg/config"
    "do-restore-service/internal/restore"
)

func main() {
    // Load configuration from environment variables
    cfg, err := config.LoadDORestoreConfig()
    if err != nil {
        fmt.Printf("Failed to load configuration: %v\n", err)
        os.Exit(1)
    }

    // Create restore service
    service, err := restore.NewRestoreService(cfg)
    if err != nil {
        fmt.Printf("Failed to create restore service: %v\n", err)
        os.Exit(1)
    }

    // Run restore once
    ctx := context.Background()
    if err := service.RunOnce(ctx); err != nil {
        fmt.Printf("Restore failed: %v\n", err)
        os.Exit(1)
    }
}