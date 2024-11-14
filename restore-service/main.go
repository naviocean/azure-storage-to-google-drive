package main

import (
    "context"
    "flag"
    "log"
    "os"
    "time"

    "restore-service/internal/config"
    "restore-service/internal/restore"
)

func main() {
    // Parse command line flags
    backupDate := flag.String("date", "", "Specific backup date to restore (format: YYYY-MM-DD)")
    flag.Parse()

    // Load configuration
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }

    // Create restore service
    service, err := restore.NewRestoreService(cfg)
    if err != nil {
        log.Fatalf("Failed to create restore service: %v", err)
    }

    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
    defer cancel()

    // Start restore process
    var restoreErr error
    if *backupDate != "" {
        // Restore specific backup
        t, err := time.Parse("2006-01-02", *backupDate)
        if err != nil {
            log.Fatalf("Invalid date format. Use YYYY-MM-DD: %v", err)
        }
        restoreErr = service.RestoreFromDate(ctx, t)
    } else {
        // Restore latest backup
        restoreErr = service.RestoreLatest(ctx)
    }

    if restoreErr != nil {
        log.Fatalf("Restore failed: %v", restoreErr)
    }

    os.Exit(0)
}