package usecases

import (
	"context"
	"fmt"
	"io"

	"todo-service/internal/domain/entities"
	"todo-service/internal/domain/ports"
)

type FileUseCase struct {
	fileStorage ports.FileStorage
}

func NewFileUseCase(fileStorage ports.FileStorage) *FileUseCase {
	return &FileUseCase{
		fileStorage: fileStorage,
	}
}

type UploadFileRequest struct {
	FileName    string
	ContentType string
	Data        io.Reader
	Size        int64
}

type UploadFileResponse struct {
	FileID string `json:"file_id"`
}

func (uc *FileUseCase) UploadFile(ctx context.Context, req UploadFileRequest) (*UploadFileResponse, error) {
	if err := entities.ValidateFile(req.FileName, req.Size); err != nil {
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	file := entities.NewFile(req.FileName, req.ContentType, req.Size)

	if !file.IsValid() {
		return nil, fmt.Errorf("invalid file data")
	}

	if err := uc.fileStorage.UploadFile(ctx, file.StoragePath, req.ContentType, req.Data, req.Size); err != nil {
		return nil, fmt.Errorf("failed to upload file to storage: %w", err)
	}

	return &UploadFileResponse{
		FileID: file.ID.String(),
	}, nil
}
