package usecases

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"todo-service/internal/domain/ports"
	"todo-service/internal/domain/ports/mocks"
)

func TestCreateTodo(t *testing.T) {
	mockTxManager := mocks.NewMockTransactionManager(t)
	mockPublisher := mocks.NewMockStreamPublisher(t)

	mockTxManager.EXPECT().DoInTx(mock.Anything, mock.AnythingOfType("func(ports.TodoRepository) error")).
		RunAndReturn(func(ctx context.Context, fn func(repo ports.TodoRepository) error) error {
			mockRepo := mocks.NewMockTodoRepository(t)
			mockRepo.EXPECT().Create(mock.Anything, mock.AnythingOfType("*entities.TodoItem")).Return(nil)
			return fn(mockRepo)
		})

	mockPublisher.EXPECT().PublishTodoCreated(mock.Anything, mock.AnythingOfType("*entities.TodoItem")).Return(nil)

	useCase := NewTodoUseCase(mockTxManager, mockPublisher)

	dueDate := time.Now().Add(24 * time.Hour)
	fileID := "test-file-id"
	req := CreateTodoRequest{
		Description: "Test Todo",
		DueDate:     dueDate,
		FileID:      &fileID,
	}

	todo, err := useCase.CreateTodo(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, req.Description, todo.Description)
	assert.True(t, todo.DueDate.Equal(req.DueDate))
	assert.NotNil(t, todo.FileID)
	assert.Equal(t, *req.FileID, *todo.FileID)
}

func TestCreateTodoWithRedisFailureRollback(t *testing.T) {
	mockTxManager := mocks.NewMockTransactionManager(t)
	mockPublisher := mocks.NewMockStreamPublisher(t)

	mockTxManager.EXPECT().DoInTx(mock.Anything, mock.AnythingOfType("func(ports.TodoRepository) error")).
		RunAndReturn(func(ctx context.Context, fn func(repo ports.TodoRepository) error) error {
			mockRepo := mocks.NewMockTodoRepository(t)
			mockRepo.EXPECT().Create(mock.Anything, mock.AnythingOfType("*entities.TodoItem")).Return(nil)
			return fn(mockRepo)
		})

	mockPublisher.EXPECT().PublishTodoCreated(mock.Anything, mock.AnythingOfType("*entities.TodoItem")).
		Return(assert.AnError)

	useCase := NewTodoUseCase(mockTxManager, mockPublisher)

	dueDate := time.Now().Add(24 * time.Hour)
	req := CreateTodoRequest{
		Description: "Test Todo",
		DueDate:     dueDate,
		FileID:      nil,
	}

	_, err := useCase.CreateTodo(context.Background(), req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create todo")
}

func TestCreateTodoWithTransactionFailure(t *testing.T) {
	mockTxManager := mocks.NewMockTransactionManager(t)
	mockPublisher := mocks.NewMockStreamPublisher(t)

	mockTxManager.EXPECT().DoInTx(mock.Anything, mock.AnythingOfType("func(ports.TodoRepository) error")).
		Return(assert.AnError)

	useCase := NewTodoUseCase(mockTxManager, mockPublisher)

	dueDate := time.Now().Add(24 * time.Hour)
	req := CreateTodoRequest{
		Description: "Test Todo",
		DueDate:     dueDate,
		FileID:      nil,
	}

	_, err := useCase.CreateTodo(context.Background(), req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create todo")
}

func TestCreateTodoWithInvalidData(t *testing.T) {
	mockTxManager := mocks.NewMockTransactionManager(t)
	mockPublisher := mocks.NewMockStreamPublisher(t)

	useCase := NewTodoUseCase(mockTxManager, mockPublisher)

	req := CreateTodoRequest{
		Description: "",
		DueDate:     time.Now().Add(24 * time.Hour),
		FileID:      nil,
	}

	_, err := useCase.CreateTodo(context.Background(), req)

	assert.Error(t, err)
	assert.Equal(t, "invalid todo item: description is required", err.Error())
}

func TestUploadFile(t *testing.T) {
	mockStorage := mocks.NewMockFileStorage(t)

	mockStorage.EXPECT().UploadFile(
		mock.Anything,
		mock.MatchedBy(func(storagePath string) bool {
			return strings.HasPrefix(storagePath, "files/") && strings.HasSuffix(storagePath, "/test.txt")
		}),
		"text/plain",
		mock.Anything,
		int64(1024),
	).Return(nil)

	useCase := NewFileUseCase(mockStorage)

	req := UploadFileRequest{
		FileName:    "test.txt",
		ContentType: "text/plain",
		Data:        strings.NewReader("test content"),
		Size:        1024,
	}

	response, err := useCase.UploadFile(context.Background(), req)

	assert.NoError(t, err)
	assert.NotEmpty(t, response.FileID)
}

func TestUploadFileWithInvalidData(t *testing.T) {
	mockStorage := mocks.NewMockFileStorage(t)

	useCase := NewFileUseCase(mockStorage)

	req := UploadFileRequest{
		FileName:    "test.exe",
		ContentType: "application/exe",
		Data:        strings.NewReader("test content"),
		Size:        1024,
	}

	_, err := useCase.UploadFile(context.Background(), req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file validation failed")
	assert.Contains(t, err.Error(), "file type .exe is not allowed")
}

func TestUploadFileWithStorageFailure(t *testing.T) {
	mockStorage := mocks.NewMockFileStorage(t)

	mockStorage.EXPECT().UploadFile(
		mock.Anything,
		mock.MatchedBy(func(storagePath string) bool {
			return strings.HasPrefix(storagePath, "files/") && strings.HasSuffix(storagePath, "/test.txt")
		}),
		"text/plain",
		mock.Anything,
		int64(1024),
	).Return(assert.AnError)

	useCase := NewFileUseCase(mockStorage)

	req := UploadFileRequest{
		FileName:    "test.txt",
		ContentType: "text/plain",
		Data:        strings.NewReader("test content"),
		Size:        1024,
	}

	_, err := useCase.UploadFile(context.Background(), req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to upload file to storage")
}
