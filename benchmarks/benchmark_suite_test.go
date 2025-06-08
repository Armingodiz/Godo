package benchmarks

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"todo-service/internal/config"
	"todo-service/internal/domain/entities"
	"todo-service/internal/domain/ports"
	"todo-service/internal/infrastructure/repositories"
)

func BenchmarkFullWorkflow(b *testing.B) {
	cfg := setupConfig()
	db := setupMySQLConnection(b, cfg)
	defer db.Close()
	defer cleanupMySQLTestData(b, db)

	s3Storage := setupS3Storage(b)
	defer cleanupS3TestData(b, s3Storage)

	publisher := setupRedisStreamPublisher(b)
	defer cleanupRedisTestData(b, publisher)

	txManager := setupTransactionManager(db)
	ctx := context.Background()

	workflows := make([]workflowData, b.N)
	for i := 0; i < b.N; i++ {
		fileID := uuid.New().String()
		workflows[i] = workflowData{
			todo: entities.NewTodoItem(
				fmt.Sprintf("Full workflow benchmark todo %d", i),
				time.Now().Add(24*time.Hour),
				&fileID,
			),
			fileData:    generateTestData(1024),
			storagePath: fmt.Sprintf("benchmark/workflow-file-%d.txt", i),
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		workflow := workflows[i]

		err := txManager.DoInTx(ctx, func(repo ports.TodoRepository) error {
			return repo.Create(ctx, workflow.todo)
		})
		if err != nil {
			b.Fatalf("Failed to insert todo: %v", err)
		}

		reader := bytes.NewReader(workflow.fileData)
		err = s3Storage.UploadFile(ctx, workflow.storagePath, "text/plain", reader, int64(len(workflow.fileData)))
		if err != nil {
			b.Fatalf("Failed to upload file: %v", err)
		}

		err = publisher.PublishTodoCreated(ctx, workflow.todo)
		if err != nil {
			b.Fatalf("Failed to publish message: %v", err)
		}
	}
}

func BenchmarkOperationComparison(b *testing.B) {
	cfg := setupConfig()
	db := setupMySQLConnection(b, cfg)
	defer db.Close()
	defer cleanupMySQLTestData(b, db)

	s3Storage := setupS3Storage(b)
	defer cleanupS3TestData(b, s3Storage)

	publisher := setupRedisStreamPublisher(b)
	defer cleanupRedisTestData(b, publisher)

	txManager := setupTransactionManager(db)
	ctx := context.Background()

	testData := generateTestData(1024)

	b.Run("MySQL_Insert", func(b *testing.B) {
		todos := make([]*entities.TodoItem, b.N)
		for i := 0; i < b.N; i++ {
			todos[i] = entities.NewTodoItem(
				fmt.Sprintf("Comparison MySQL todo %d", i),
				time.Now().Add(24*time.Hour),
				nil,
			)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := txManager.DoInTx(ctx, func(repo ports.TodoRepository) error {
				return repo.Create(ctx, todos[i])
			})
			if err != nil {
				b.Fatalf("Failed to insert todo: %v", err)
			}
		}
	})

	b.Run("S3_Upload", func(b *testing.B) {
		b.ResetTimer()
		b.SetBytes(int64(len(testData)))

		for i := 0; i < b.N; i++ {
			storagePath := fmt.Sprintf("benchmark/comparison-file-%d.txt", i)
			reader := bytes.NewReader(testData)

			err := s3Storage.UploadFile(ctx, storagePath, "text/plain", reader, int64(len(testData)))
			if err != nil {
				b.Fatalf("Failed to upload file: %v", err)
			}
		}
	})

	b.Run("Redis_Publish", func(b *testing.B) {
		todos := make([]*entities.TodoItem, b.N)
		for i := 0; i < b.N; i++ {
			todos[i] = entities.NewTodoItem(
				fmt.Sprintf("Comparison Redis todo %d", i),
				time.Now().Add(24*time.Hour),
				nil,
			)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := publisher.PublishTodoCreated(ctx, todos[i])
			if err != nil {
				b.Fatalf("Failed to publish message: %v", err)
			}
		}
	})
}

func BenchmarkErrorRecovery(b *testing.B) {
	cfg := setupConfig()
	db := setupMySQLConnection(b, cfg)
	defer db.Close()
	defer cleanupMySQLTestData(b, db)

	s3Storage := setupS3Storage(b)
	defer cleanupS3TestData(b, s3Storage)

	txManager := setupTransactionManager(db)
	ctx := context.Background()

	b.Run("MySQL_Retry_On_Conflict", func(b *testing.B) {
		baseID := uuid.New()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			todo := &entities.TodoItem{
				ID:          baseID,
				Description: fmt.Sprintf("Retry test todo %d", i),
				DueDate:     time.Now().Add(24 * time.Hour),
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			_ = txManager.DoInTx(ctx, func(repo ports.TodoRepository) error {
				return repo.Create(ctx, todo)
			})
		}
	})

	b.Run("S3_Upload_With_Retries", func(b *testing.B) {
		testData := generateTestData(1024)

		b.ResetTimer()
		b.SetBytes(1024)

		for i := 0; i < b.N; i++ {
			storagePath := fmt.Sprintf("benchmark/retry-test-%d.txt", i)
			reader := bytes.NewReader(testData)

			maxRetries := 3
			var err error
			for retry := 0; retry < maxRetries; retry++ {
				err = s3Storage.UploadFile(ctx, storagePath, "text/plain", reader, 1024)
				if err == nil {
					break
				}
				reader = bytes.NewReader(testData)
			}

			if err != nil {
				b.Fatalf("Failed to upload file after retries: %v", err)
			}
		}
	})
}

func BenchmarkCleanupAll(b *testing.B) {
	cfg := setupConfig()
	db := setupMySQLConnection(b, cfg)
	defer db.Close()

	s3Storage := setupS3Storage(b)
	publisher := setupRedisStreamPublisher(b)

	ctx := context.Background()
	txManager := setupTransactionManager(db)

	for i := 0; i < 10; i++ {
		todo := entities.NewTodoItem(
			fmt.Sprintf("Cleanup test todo %d", i),
			time.Now().Add(24*time.Hour),
			nil,
		)
		_ = txManager.DoInTx(ctx, func(repo ports.TodoRepository) error {
			return repo.Create(ctx, todo)
		})
	}

	testData := generateTestData(1024)
	for i := 0; i < 5; i++ {
		storagePath := fmt.Sprintf("benchmark/cleanup-test-%d.txt", i)
		reader := bytes.NewReader(testData)
		_ = s3Storage.UploadFile(ctx, storagePath, "text/plain", reader, 1024)
	}

	for i := 0; i < 5; i++ {
		todo := entities.NewTodoItem(
			fmt.Sprintf("Cleanup Redis test todo %d", i),
			time.Now().Add(24*time.Hour),
			nil,
		)
		_ = publisher.PublishTodoCreated(ctx, todo)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cleanupMySQLTestData(b, db)
		cleanupS3TestData(b, s3Storage)
		cleanupRedisTestData(b, publisher)
	}
}

type workflowData struct {
	todo        *entities.TodoItem
	fileData    []byte
	storagePath string
}

func setupConfig() *config.Config {
	return config.Load()
}

func setupTransactionManager(db *sql.DB) *repositories.MySQLTransactionManager {
	return repositories.NewMySQLTransactionManager(db)
}
