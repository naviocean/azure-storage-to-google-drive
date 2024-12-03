package utils

import (
    "fmt"
    "io"
)

// ProgressReader wraps an io.Reader to provide progress updates
type ProgressReader struct {
    io.Reader
    Total     int64
    Uploaded  int64
    OnProgress func(uploaded, total int64)
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
    n, err := pr.Reader.Read(p)
    pr.Uploaded += int64(n)
    if pr.OnProgress != nil {
        pr.OnProgress(pr.Uploaded, pr.Total)
    }
    return n, err
}

// FormatBytes converts bytes to human readable string format
func FormatBytes(bytes int64) string {
    const unit = 1024
    if bytes < unit {
        return fmt.Sprintf("%d B", bytes)
    }
    div, exp := int64(unit), 0
    for n := bytes / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}