package benchmarks

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"todo-service/internal/config"
	"todo-service/internal/domain/entities"
	"todo-service/internal/domain/ports"
	"todo-service/internal/infrastructure/repositories"
)

func BenchmarkMySQLTodoInsert(b *testing.B) {
	cfg := config.Load()
	db := setupMySQLConnection(b, cfg)
	defer db.Close()
	defer cleanupMySQLTestData(b, db)

	txManager := repositories.NewMySQLTransactionManager(db)
	ctx := context.Background()

	todos := make([]*entities.TodoItem, b.N)
	for i := 0; i < b.N; i++ {
		todos[i] = entities.NewTodoItem(
			fmt.Sprintf("Benchmark todo item %d", i),
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
}

func BenchmarkMySQLTodoInsertWithFile(b *testing.B) {
	cfg := config.Load()
	db := setupMySQLConnection(b, cfg)
	defer db.Close()
	defer cleanupMySQLTestData(b, db)

	txManager := repositories.NewMySQLTransactionManager(db)
	ctx := context.Background()

	todos := make([]*entities.TodoItem, b.N)
	for i := 0; i < b.N; i++ {
		fileID := uuid.New().String()
		todos[i] = entities.NewTodoItem(
			fmt.Sprintf("Benchmark todo with file %d", i),
			time.Now().Add(24*time.Hour),
			&fileID,
		)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := txManager.DoInTx(ctx, func(repo ports.TodoRepository) error {
			return repo.Create(ctx, todos[i])
		})
		if err != nil {
			b.Fatalf("Failed to insert todo with file: %v", err)
		}
	}
}

func setupMySQLConnection(b *testing.B, cfg *config.Config) *sql.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		cfg.DB.User,
		cfg.DB.Password,
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Name,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		b.Fatalf("Failed to connect to MySQL: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		b.Fatalf("Failed to ping MySQL: %v", err)
	}

	cleanupMySQLTestData(b, db)

	return db
}

func cleanupMySQLTestData(b *testing.B, db *sql.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queries := []string{
		"DELETE FROM todos WHERE description LIKE '%Benchmark%'",
		"DELETE FROM todos WHERE description LIKE '%benchmark%'",
		"DELETE FROM todos WHERE description LIKE '%test%'",
		"DELETE FROM todos WHERE description LIKE '%Test%'",
		"DELETE FROM todos WHERE description LIKE '%Comparison%'",
		"DELETE FROM todos WHERE description LIKE '%Retry%'",
		"DELETE FROM todos WHERE description LIKE '%workflow%'",
		"DELETE FROM todos WHERE description LIKE '%Workflow%'",
		"DELETE FROM todos WHERE description LIKE '%Concurrent%'",
		"DELETE FROM todos WHERE description LIKE '%Timeout%'",
		"DELETE FROM todos WHERE description LIKE '%Scalability%'",
		"DELETE FROM todos WHERE description LIKE '%Load%'",
	}

	for _, query := range queries {
		result, err := db.ExecContext(ctx, query)
		if err != nil {
			b.Logf("Warning: Failed to cleanup MySQL test data with query '%s': %v", query, err)
			continue
		}

		if rowsAffected, err := result.RowsAffected(); err == nil && rowsAffected > 0 {
			b.Logf("Cleaned up %d MySQL test records with query: %s", rowsAffected, query)
		}
	}

	_, err := db.ExecContext(ctx, "DELETE FROM todos WHERE created_at > DATE_SUB(NOW(), INTERVAL 1 HOUR)")
	if err != nil {
		b.Logf("Warning: Failed to cleanup recent MySQL test data: %v", err)
	}
}
