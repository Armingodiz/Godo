package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"todo-service/internal/domain/entities"
	"todo-service/internal/domain/ports"
	"todo-service/internal/domain/ports/mocks"
)

func TestFileStorage_UploadWithSpecificExpectations(t *testing.T) {
	mockStorage := mocks.NewMockFileStorage(t)

	mockStorage.EXPECT().UploadFile(
		mock.MatchedBy(func(ctx context.Context) bool {
			_, hasDeadline := ctx.Deadline()
			return hasDeadline
		}),
		mock.MatchedBy(func(storagePath string) bool {
			return strings.HasPrefix(storagePath, "files/") && strings.HasSuffix(storagePath, "/document.pdf")
		}),
		"application/pdf",
		mock.MatchedBy(func(data interface{}) bool {
			return data != nil
		}),
		int64(2048),
	).Return(nil).Once()

	useCase := NewFileUseCase(mockStorage)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := UploadFileRequest{
		FileName:    "document.pdf",
		ContentType: "application/pdf",
		Data:        strings.NewReader("fake pdf content"),
		Size:        2048,
	}

	response, err := useCase.UploadFile(ctx, req)

	assert.NoError(t, err)
	assert.NotEmpty(t, response.FileID)
}

func TestFileStorage_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*mocks.MockFileStorage)
		expectedError string
	}{
		{
			name: "S3 connection timeout",
			setupMock: func(m *mocks.MockFileStorage) {
				m.EXPECT().UploadFile(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("context deadline exceeded"))
			},
			expectedError: "failed to upload file to storage: context deadline exceeded",
		},
		{
			name: "S3 access denied",
			setupMock: func(m *mocks.MockFileStorage) {
				m.EXPECT().UploadFile(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("access denied"))
			},
			expectedError: "failed to upload file to storage: access denied",
		},
		{
			name: "S3 bucket not found",
			setupMock: func(m *mocks.MockFileStorage) {
				m.EXPECT().UploadFile(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("bucket does not exist"))
			},
			expectedError: "failed to upload file to storage: bucket does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := mocks.NewMockFileStorage(t)
			tt.setupMock(mockStorage)

			useCase := NewFileUseCase(mockStorage)

			req := UploadFileRequest{
				FileName:    "test.txt",
				ContentType: "text/plain",
				Data:        strings.NewReader("test content"),
				Size:        1024,
			}

			_, err := useCase.UploadFile(context.Background(), req)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestStreamPublisher_PublishWithSpecificExpectations(t *testing.T) {
	mockTxManager := mocks.NewMockTransactionManager(t)
	mockPublisher := mocks.NewMockStreamPublisher(t)

	mockTxManager.EXPECT().DoInTx(mock.Anything, mock.AnythingOfType("func(ports.TodoRepository) error")).
		RunAndReturn(func(ctx context.Context, fn func(repo ports.TodoRepository) error) error {
			mockRepo := mocks.NewMockTodoRepository(t)
			mockRepo.EXPECT().Create(mock.Anything, mock.AnythingOfType("*entities.TodoItem")).Return(nil)
			return fn(mockRepo)
		})

	mockPublisher.EXPECT().PublishTodoCreated(
		mock.MatchedBy(func(ctx context.Context) bool {
			return ctx != nil
		}),
		mock.MatchedBy(func(todo *entities.TodoItem) bool {
			return todo != nil &&
				todo.Description == "Important Task" &&
				todo.FileID != nil &&
				*todo.FileID == "file-123"
		}),
	).Return(nil).Once()

	useCase := NewTodoUseCase(mockTxManager, mockPublisher)

	fileID := "file-123"
	req := CreateTodoRequest{
		Description: "Important Task",
		DueDate:     time.Now().Add(24 * time.Hour),
		FileID:      &fileID,
	}

	todo, err := useCase.CreateTodo(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "Important Task", todo.Description)
	assert.Equal(t, "file-123", *todo.FileID)
}

func TestStreamPublisher_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockTransactionManager, *mocks.MockStreamPublisher)
		expectedError string
	}{
		{
			name: "Redis connection lost",
			setupMocks: func(txManager *mocks.MockTransactionManager, publisher *mocks.MockStreamPublisher) {
				txManager.EXPECT().DoInTx(mock.Anything, mock.AnythingOfType("func(ports.TodoRepository) error")).
					RunAndReturn(func(ctx context.Context, fn func(repo ports.TodoRepository) error) error {
						mockRepo := mocks.NewMockTodoRepository(t)
						mockRepo.EXPECT().Create(mock.Anything, mock.AnythingOfType("*entities.TodoItem")).Return(nil)
						return fn(mockRepo)
					})
				publisher.EXPECT().PublishTodoCreated(mock.Anything, mock.Anything).
					Return(errors.New("redis: connection refused"))
			},
			expectedError: "failed to create todo: redis: connection refused",
		},
		{
			name: "Redis stream full",
			setupMocks: func(txManager *mocks.MockTransactionManager, publisher *mocks.MockStreamPublisher) {
				txManager.EXPECT().DoInTx(mock.Anything, mock.AnythingOfType("func(ports.TodoRepository) error")).
					RunAndReturn(func(ctx context.Context, fn func(repo ports.TodoRepository) error) error {
						mockRepo := mocks.NewMockTodoRepository(t)
						mockRepo.EXPECT().Create(mock.Anything, mock.AnythingOfType("*entities.TodoItem")).Return(nil)
						return fn(mockRepo)
					})
				publisher.EXPECT().PublishTodoCreated(mock.Anything, mock.Anything).
					Return(errors.New("stream length exceeded"))
			},
			expectedError: "failed to create todo: stream length exceeded",
		},
		{
			name: "Redis authentication failed",
			setupMocks: func(txManager *mocks.MockTransactionManager, publisher *mocks.MockStreamPublisher) {
				txManager.EXPECT().DoInTx(mock.Anything, mock.AnythingOfType("func(ports.TodoRepository) error")).
					RunAndReturn(func(ctx context.Context, fn func(repo ports.TodoRepository) error) error {
						mockRepo := mocks.NewMockTodoRepository(t)
						mockRepo.EXPECT().Create(mock.Anything, mock.AnythingOfType("*entities.TodoItem")).Return(nil)
						return fn(mockRepo)
					})
				publisher.EXPECT().PublishTodoCreated(mock.Anything, mock.Anything).
					Return(errors.New("NOAUTH Authentication required"))
			},
			expectedError: "failed to create todo: NOAUTH Authentication required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTxManager := mocks.NewMockTransactionManager(t)
			mockPublisher := mocks.NewMockStreamPublisher(t)

			tt.setupMocks(mockTxManager, mockPublisher)

			useCase := NewTodoUseCase(mockTxManager, mockPublisher)

			req := CreateTodoRequest{
				Description: "Test Todo",
				DueDate:     time.Now().Add(24 * time.Hour),
				FileID:      nil,
			}

			_, err := useCase.CreateTodo(context.Background(), req)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestComplexScenario_FileUploadAndTodoCreation(t *testing.T) {

	mockStorage := mocks.NewMockFileStorage(t)
	mockTxManager := mocks.NewMockTransactionManager(t)
	mockPublisher := mocks.NewMockStreamPublisher(t)

	mockStorage.EXPECT().UploadFile(
		mock.Anything,
		mock.MatchedBy(func(storagePath string) bool {
			return strings.HasPrefix(storagePath, "files/") && strings.HasSuffix(storagePath, "/report.pdf")
		}),
		"application/pdf",
		mock.Anything,
		int64(5120),
	).Return(nil).Once()

	mockTxManager.EXPECT().DoInTx(mock.Anything, mock.AnythingOfType("func(ports.TodoRepository) error")).
		RunAndReturn(func(ctx context.Context, fn func(repo ports.TodoRepository) error) error {
			mockRepo := mocks.NewMockTodoRepository(t)
			mockRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(todo *entities.TodoItem) bool {
				return todo.Description == "Review uploaded report" && todo.FileID != nil
			})).Return(nil)
			return fn(mockRepo)
		}).Once()

	mockPublisher.EXPECT().PublishTodoCreated(
		mock.Anything,
		mock.MatchedBy(func(todo *entities.TodoItem) bool {
			return todo.Description == "Review uploaded report" && todo.FileID != nil
		}),
	).Return(nil).Once()

	fileUseCase := NewFileUseCase(mockStorage)
	todoUseCase := NewTodoUseCase(mockTxManager, mockPublisher)

	uploadReq := UploadFileRequest{
		FileName:    "report.pdf",
		ContentType: "application/pdf",
		Data:        strings.NewReader("fake pdf report content"),
		Size:        5120,
	}

	uploadResp, err := fileUseCase.UploadFile(context.Background(), uploadReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, uploadResp.FileID)

	todoReq := CreateTodoRequest{
		Description: "Review uploaded report",
		DueDate:     time.Now().Add(48 * time.Hour),
		FileID:      &uploadResp.FileID,
	}

	todo, err := todoUseCase.CreateTodo(context.Background(), todoReq)
	assert.NoError(t, err)
	assert.Equal(t, "Review uploaded report", todo.Description)
	assert.Equal(t, uploadResp.FileID, *todo.FileID)
}

func TestCustomMatchers(t *testing.T) {
	mockStorage := mocks.NewMockFileStorage(t)

	storagePathMatcher := mock.MatchedBy(func(path string) bool {
		parts := strings.Split(path, "/")
		if len(parts) != 3 {
			return false
		}
		if parts[0] != "files" {
			return false
		}
		if len(parts[1]) != 36 {
			return false
		}
		return parts[2] == "document.txt"
	})

	contentTypeMatcher := mock.MatchedBy(func(contentType string) bool {
		return contentType == "text/plain"
	})

	mockStorage.EXPECT().UploadFile(
		mock.Anything,
		storagePathMatcher,
		contentTypeMatcher,
		mock.Anything,
		int64(1024),
	).Return(nil)

	useCase := NewFileUseCase(mockStorage)

	req := UploadFileRequest{
		FileName:    "document.txt",
		ContentType: "text/plain",
		Data:        strings.NewReader("document content"),
		Size:        1024,
	}

	response, err := useCase.UploadFile(context.Background(), req)

	assert.NoError(t, err)
	assert.NotEmpty(t, response.FileID)
}
