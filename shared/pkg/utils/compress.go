package utils

import (
    "archive/zip"
    "fmt"
    "io"
    "os"
    "path/filepath"
)

func ZipDirectory(source, target string) error {
    zipfile, err := os.Create(target)
    if err != nil {
        return fmt.Errorf("failed to create zip file: %v", err)
    }
    defer zipfile.Close()

    archive := zip.NewWriter(zipfile)
    defer archive.Close()

    // Walk through the directory tree
    err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return fmt.Errorf("error walking directory: %v", err)
        }

        // Create zip header
        header, err := zip.FileInfoHeader(info)
        if err != nil {
            return fmt.Errorf("failed to create zip header: %v", err)
        }

        // Ensure consistent paths on Windows and Unix
        relPath, err := filepath.Rel(source, path)
        if err != nil {
            return fmt.Errorf("failed to get relative path: %v", err)
        }
        if relPath == "." {
            return nil
        }
        header.Name = filepath.ToSlash(relPath)

        if info.IsDir() {
            header.Name += "/"
        } else {
            header.Method = zip.Deflate
        }

        writer, err := archive.CreateHeader(header)
        if err != nil {
            return fmt.Errorf("failed to create zip entry: %v", err)
        }

        if !info.IsDir() {
            file, err := os.Open(path)
            if err != nil {
                return fmt.Errorf("failed to open file: %v", err)
            }
            defer file.Close()

            _, err = io.Copy(writer, file)
            if err != nil {
                return fmt.Errorf("failed to write file to zip: %v", err)
            }
        }

        return nil
    })

    return err
}

func UnzipFile(zipPath, destPath string) error {
    reader, err := zip.OpenReader(zipPath)
    if err != nil {
        return fmt.Errorf("failed to open zip file: %v", err)
    }
    defer reader.Close()

    if err := os.MkdirAll(destPath, 0755); err != nil {
        return fmt.Errorf("failed to create destination directory: %v", err)
    }

    for _, file := range reader.File {
        err := extractFile(file, destPath)
        if err != nil {
            return fmt.Errorf("failed to extract file %s: %v", file.Name, err)
        }
    }

    return nil
}

func extractFile(file *zip.File, destPath string) error {
    filePath := filepath.Join(destPath, file.Name)

    if file.FileInfo().IsDir() {
        if err := os.MkdirAll(filePath, file.Mode()); err != nil {
            return fmt.Errorf("failed to create directory: %v", err)
        }
        return nil
    }

    if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
        return fmt.Errorf("failed to create parent directory: %v", err)
    }

    // Create temp file
    tempPath := filePath + ".tmp"
    dest, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
    if err != nil {
        return fmt.Errorf("failed to create destination file: %v", err)
    }

    src, err := file.Open()
    if err != nil {
        dest.Close()
        os.Remove(tempPath)
        return fmt.Errorf("failed to open source file: %v", err)
    }

    _, err = io.Copy(dest, src)
    src.Close()
    dest.Close()

    if err != nil {
        os.Remove(tempPath)
        return fmt.Errorf("failed to extract file content: %v", err)
    }

    // Atomic rename
    if err := os.Rename(tempPath, filePath); err != nil {
        os.Remove(tempPath)
        return fmt.Errorf("failed to rename temp file: %v", err)
    }

    return nil
}