package ports

import (
	"context"
	"io"

	"todo-service/internal/domain/entities"
)

type TodoRepository interface {
	Create(ctx context.Context, todo *entities.TodoItem) error
}

type TransactionManager interface {
	DoInTx(ctx context.Context, fn func(repo TodoRepository) error) error
}

type StreamPublisher interface {
	PublishTodoCreated(ctx context.Context, todo *entities.TodoItem) error
}

type FileStorage interface {
	UploadFile(ctx context.Context, storagePath, contentType string, data io.Reader, size int64) error
	EnsureBucket(ctx context.Context) error
}
