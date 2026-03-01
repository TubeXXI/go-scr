package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func SanitizeFilename(filename string) string {
	reg := regexp.MustCompile(`[<>:"/\\|?*]`)
	sanitized := reg.ReplaceAllString(filename, "")

	if len(sanitized) > 200 {
		sanitized = sanitized[:200]
	}

	return strings.TrimSpace(sanitized)
}

func EnsureDir(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return os.MkdirAll(dirPath, 0755)
	}
	return nil
}

func GetDownloadPath(baseDir, filename, ext string) (string, error) {
	if err := EnsureDir(baseDir); err != nil {
		return "", err
	}

	sanitized := SanitizeFilename(filename)
	fullPath := filepath.Join(baseDir, sanitized+ext)

	// Jika file sudah ada, tambahkan counter
	counter := 1
	for {
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			break
		}
		fullPath = filepath.Join(baseDir, fmt.Sprintf("%s_%d%s", sanitized, counter, ext))
		counter++
	}

	return fullPath, nil
}

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

func GetFileExtension(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

func IsValidPath(path string) bool {
	_, err := filepath.Abs(path)
	return err == nil
}

func ClearAllFilesInDir(dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			err := os.Remove(filepath.Join(dirPath, entry.Name()))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
