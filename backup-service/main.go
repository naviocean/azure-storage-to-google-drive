package main

import (
    "flag"
    "log"
    "os"
    "os/signal"
    "syscall"

    "shared/pkg/config"
    "backup-service/internal/backup"
)

func main() {
    // Parse command line flags
    listFolders := flag.Bool("list-folders", false, "List available folders in Shared Drive")
    flag.Parse()

    // Load configuration
    cfg, err := config.LoadBackupConfig()
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }

    // Create backup service
    service, err := backup.NewBackupService(cfg)
    if err != nil {
        log.Fatalf("Failed to create backup service: %v", err)
    }

    // If list-folders flag is set, just list folders and exit
    if *listFolders {
        if err := service.ListFolders(); err != nil {
            log.Fatalf("Failed to list folders: %v", err)
        }
        return
    }

    // Start scheduler
    if err := service.StartScheduler(); err != nil {
        log.Fatalf("Failed to start scheduler: %v", err)
    }

    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    log.Println("Shutting down...")
}