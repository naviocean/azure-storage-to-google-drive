package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "backup-service/internal/backup"
    "backup-service/internal/config"
)

func main() {
    // Load configuration
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }

    // Create backup service
    service, err := backup.NewBackupService(cfg)
    if err != nil {
        log.Fatalf("Failed to create backup service: %v", err)
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