package utils

import (
    "log"
    "os"
    "sync"
)

type LogLevel int

const (
    DEBUG LogLevel = iota
    INFO
    WARN
    ERROR
)

type Logger struct {
    *log.Logger
    level LogLevel
    mu    sync.Mutex
}

func NewLogger(prefix string, levelStr string) *Logger {
    level := parseLogLevel(levelStr)
    return &Logger{
        Logger: log.New(os.Stdout, prefix+" ", log.LstdFlags|log.Lmsgprefix),
        level:  level,
    }
}

func parseLogLevel(level string) LogLevel {
    switch level {
    case "debug":
        return DEBUG
    case "info":
        return INFO
    case "warn":
        return WARN
    case "error":
        return ERROR
    default:
        return INFO
    }
}

func (l *Logger) Debug(format string, v ...interface{}) {
    if l.level <= DEBUG {
        l.mu.Lock()
        l.Printf("[DEBUG] "+format, v...)
        l.mu.Unlock()
    }
}

func (l *Logger) Info(format string, v ...interface{}) {
    if l.level <= INFO {
        l.mu.Lock()
        l.Printf("[INFO] "+format, v...)
        l.mu.Unlock()
    }
}

func (l *Logger) Warn(format string, v ...interface{}) {
    if l.level <= WARN {
        l.mu.Lock()
        l.Printf("[WARN] "+format, v...)
        l.mu.Unlock()
    }
}

func (l *Logger) Error(format string, v ...interface{}) {
    if l.level <= ERROR {
        l.mu.Lock()
        l.Printf("[ERROR] "+format, v...)
        l.mu.Unlock()
    }
}