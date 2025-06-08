package usecases

import (
	"context"
	"fmt"
	"time"

	"todo-service/internal/domain/entities"
	"todo-service/internal/domain/ports"
)

type TodoUseCase struct {
	todoRepo        ports.TodoRepository
	txManager       ports.TransactionManager
	streamPublisher ports.StreamPublisher
}

func NewTodoUseCase(
	todoRepo ports.TodoRepository,
	txManager ports.TransactionManager,
	streamPublisher ports.StreamPublisher,
) *TodoUseCase {
	return &TodoUseCase{
		todoRepo:        todoRepo,
		txManager:       txManager,
		streamPublisher: streamPublisher,
	}
}

type CreateTodoRequest struct {
	Description string    `json:"description" binding:"required"`
	DueDate     time.Time `json:"due_date" binding:"required"`
	FileID      *string   `json:"file_id,omitempty"`
}

func (uc *TodoUseCase) CreateTodo(ctx context.Context, req CreateTodoRequest) (*entities.TodoItem, error) {
	todo := entities.NewTodoItem(req.Description, req.DueDate, req.FileID)

	if !todo.IsValid() {
		return nil, fmt.Errorf("invalid todo item: description is required")
	}

	err := uc.txManager.DoInTx(ctx, func(repo ports.TodoRepository) error {
		if err := repo.Create(ctx, todo); err != nil {
			return err
		}

		return uc.streamPublisher.PublishTodoCreated(ctx, todo)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create todo: %w", err)
	}

	return todo, nil
}
