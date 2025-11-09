// Package helpers provides file handling utilities.
package helpers

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// FileCleanup is a resource manager for temporary files
type FileCleanup struct {
	files []string
}

// Add registers a file for cleanup
func (fc *FileCleanup) Add(path string) {
	fc.files = append(fc.files, path)
}

// Cleanup removes all registered files
func (fc *FileCleanup) Cleanup() error {
	var lastErr error
	for _, f := range fc.files {
		if err := os.Remove(f); err != nil && lastErr == nil {
			lastErr = err
			DebugLog("Failed to remove temp file %s: %v", f, err)
		}
	}
	return lastErr
}

// NewFileCleanup creates a new FileCleanup manager
func NewFileCleanup() *FileCleanup {
	return &FileCleanup{
		files: make([]string, 0),
	}
}

// SaveMultipartFile saves an uploaded multipart file to a temporary location
// Returns the path to the temporary file or error.
// The caller should add the file to a FileCleanup manager for cleanup.
func SaveMultipartFile(header *multipart.FileHeader, prefix string) (string, error) {
	file, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open uploaded file %s: %w", header.Filename, err)
	}
	defer func() { _ = file.Close() }()

	// Create temp file with UUID to avoid conflicts
	fileExt := filepath.Ext(header.Filename)
	tempFileName := filepath.Join(os.TempDir(), fmt.Sprintf("%s%s%s", prefix, uuid.New().String(), fileExt))

	tempFile, err := os.Create(tempFileName)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = tempFile.Close() }()

	// Copy file content
	if _, err := io.Copy(tempFile, file); err != nil {
		_ = os.Remove(tempFileName) // Best effort cleanup
		return "", fmt.Errorf("failed to copy uploaded file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempFileName) // Best effort cleanup
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	return tempFileName, nil
}

// SaveMultipartFileWithCleanup saves a multipart file and registers it for cleanup
func SaveMultipartFileWithCleanup(header *multipart.FileHeader, prefix string, cleanup *FileCleanup) (string, error) {
	path, err := SaveMultipartFile(header, prefix)
	if err != nil {
		return "", err
	}

	cleanup.Add(path)
	return path, nil
}

// VerifyFileNotEmpty checks that a file exists and has content
func VerifyFileNotEmpty(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	if fileInfo.Size() == 0 {
		return fmt.Errorf("file %s is empty", path)
	}

	return nil
}

// GetFileSize returns the size of a file, or -1 if it doesn't exist
func GetFileSize(path string) int64 {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return fileInfo.Size()
}
