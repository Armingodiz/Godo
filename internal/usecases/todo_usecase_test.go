package usecases

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"todo-service/internal/domain/entities"
	"todo-service/internal/domain/ports"
)

type MockTodoRepository struct {
	todos map[string]*entities.TodoItem
}

func NewMockTodoRepository() *MockTodoRepository {
	return &MockTodoRepository{
		todos: make(map[string]*entities.TodoItem),
	}
}

func (m *MockTodoRepository) Create(ctx context.Context, todo *entities.TodoItem) error {
	m.todos[todo.ID.String()] = todo
	return nil
}

type MockTransactionManager struct {
	shouldFail bool
}

func NewMockTransactionManager() *MockTransactionManager {
	return &MockTransactionManager{
		shouldFail: false,
	}
}

func (m *MockTransactionManager) DoInTx(ctx context.Context, fn func(repo ports.TodoRepository) error) error {
	if m.shouldFail {
		return errors.New("transaction failed")
	}

	// Create a mock repository for the transaction
	txRepo := NewMockTodoRepository()
	return fn(txRepo)
}

type MockStreamPublisher struct {
	publishedEvents []string
	shouldFail      bool
}

func NewMockStreamPublisher() *MockStreamPublisher {
	return &MockStreamPublisher{
		publishedEvents: make([]string, 0),
		shouldFail:      false,
	}
}

func (m *MockStreamPublisher) PublishTodoCreated(ctx context.Context, todo *entities.TodoItem) error {
	if m.shouldFail {
		return errors.New("redis connection failed")
	}
	m.publishedEvents = append(m.publishedEvents, "todo.created")
	return nil
}

type MockFileStorage struct{}

func NewMockFileStorage() *MockFileStorage {
	return &MockFileStorage{}
}

func (m *MockFileStorage) UploadFile(ctx context.Context, storagePath, contentType string, data io.Reader, size int64) error {
	return nil
}

func (m *MockFileStorage) EnsureBucket(ctx context.Context) error {
	return nil
}

func TestCreateTodo(t *testing.T) {
	mockRepo := NewMockTodoRepository()
	mockTxManager := NewMockTransactionManager()
	mockPublisher := NewMockStreamPublisher()

	useCase := NewTodoUseCase(mockRepo, mockTxManager, mockPublisher)

	dueDate := time.Now().Add(24 * time.Hour)
	fileID := "test-file-id"
	req := CreateTodoRequest{
		Description: "Test Todo",
		DueDate:     dueDate,
		FileID:      &fileID,
	}

	todo, err := useCase.CreateTodo(context.Background(), req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if todo.Description != req.Description {
		t.Errorf("Expected description %s, got %s", req.Description, todo.Description)
	}

	if !todo.DueDate.Equal(req.DueDate) {
		t.Errorf("Expected due date %v, got %v", req.DueDate, todo.DueDate)
	}

	if todo.FileID == nil || *todo.FileID != *req.FileID {
		t.Errorf("Expected file ID %s, got %v", *req.FileID, todo.FileID)
	}

	if len(mockPublisher.publishedEvents) != 1 {
		t.Errorf("Expected 1 published event, got %d", len(mockPublisher.publishedEvents))
	}

	if mockPublisher.publishedEvents[0] != "todo.created" {
		t.Errorf("Expected 'todo.created' event, got %s", mockPublisher.publishedEvents[0])
	}
}

func TestCreateTodoWithRedisFailureRollback(t *testing.T) {
	mockRepo := NewMockTodoRepository()
	mockTxManager := NewMockTransactionManager()
	mockPublisher := NewMockStreamPublisher()
	mockPublisher.shouldFail = true

	useCase := NewTodoUseCase(mockRepo, mockTxManager, mockPublisher)

	dueDate := time.Now().Add(24 * time.Hour)
	req := CreateTodoRequest{
		Description: "Test Todo",
		DueDate:     dueDate,
		FileID:      nil,
	}

	_, err := useCase.CreateTodo(context.Background(), req)

	if err == nil {
		t.Fatal("Expected error when Redis publishing fails, got nil")
	}

	if len(mockPublisher.publishedEvents) != 0 {
		t.Errorf("Expected no published events due to failure, got %d", len(mockPublisher.publishedEvents))
	}
}

func TestCreateTodoWithTransactionFailure(t *testing.T) {
	mockRepo := NewMockTodoRepository()
	mockTxManager := NewMockTransactionManager()
	mockTxManager.shouldFail = true
	mockPublisher := NewMockStreamPublisher()

	useCase := NewTodoUseCase(mockRepo, mockTxManager, mockPublisher)

	dueDate := time.Now().Add(24 * time.Hour)
	req := CreateTodoRequest{
		Description: "Test Todo",
		DueDate:     dueDate,
		FileID:      nil,
	}

	_, err := useCase.CreateTodo(context.Background(), req)

	if err == nil {
		t.Fatal("Expected error when transaction fails, got nil")
	}

	if len(mockRepo.todos) != 0 {
		t.Errorf("Expected no todos to be saved when transaction fails, but found %d todos", len(mockRepo.todos))
	}
}

func TestCreateTodoWithInvalidData(t *testing.T) {
	mockRepo := NewMockTodoRepository()
	mockTxManager := NewMockTransactionManager()
	mockPublisher := NewMockStreamPublisher()

	useCase := NewTodoUseCase(mockRepo, mockTxManager, mockPublisher)

	req := CreateTodoRequest{
		Description: "",
		DueDate:     time.Now().Add(24 * time.Hour),
		FileID:      nil,
	}

	_, err := useCase.CreateTodo(context.Background(), req)

	if err == nil {
		t.Fatal("Expected error for invalid todo, got nil")
	}

	expectedError := "invalid todo item: description is required"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestUploadFile(t *testing.T) {
	mockStorage := NewMockFileStorage()
	useCase := NewFileUseCase(mockStorage)

	req := UploadFileRequest{
		FileName:    "test.txt",
		ContentType: "text/plain",
		Data:        nil,
		Size:        1024,
	}

	response, err := useCase.UploadFile(context.Background(), req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if response.FileID == "" {
		t.Error("Expected file ID to be returned")
	}
}

func TestUploadFileWithInvalidData(t *testing.T) {
	mockStorage := NewMockFileStorage()
	useCase := NewFileUseCase(mockStorage)

	req := UploadFileRequest{
		FileName:    "test.exe",
		ContentType: "application/exe",
		Data:        nil,
		Size:        1024,
	}

	_, err := useCase.UploadFile(context.Background(), req)

	if err == nil {
		t.Fatal("Expected error for invalid file type, got nil")
	}
}
