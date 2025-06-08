package entities

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	MaxFileSize = 10 * 1024 * 1024
)

var allowedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".pdf":  true,
	".txt":  true,
	".doc":  true,
	".docx": true,
}

type File struct {
	ID          uuid.UUID `json:"id"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	StoragePath string    `json:"storage_path"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewFile(fileName, contentType string, size int64) *File {
	now := time.Now()
	id := uuid.New()
	return &File{
		ID:          id,
		FileName:    fileName,
		ContentType: contentType,
		Size:        size,
		StoragePath: generateStoragePath(id.String(), fileName),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (f *File) IsValid() bool {
	return f.FileName != "" && f.Size > 0 && f.ID != uuid.Nil
}

func ValidateFile(fileName string, size int64) error {
	if size > MaxFileSize {
		return fmt.Errorf("file size exceeds maximum allowed size of %d bytes", MaxFileSize)
	}

	if size <= 0 {
		return fmt.Errorf("file size must be greater than 0")
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	if !allowedExtensions[ext] {
		return fmt.Errorf("file type %s is not allowed", ext)
	}

	if fileName == "" {
		return fmt.Errorf("file name cannot be empty")
	}

	return nil
}

func generateStoragePath(fileID, fileName string) string {
	return "files/" + fileID + "/" + fileName
}
