package repositories

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"todo-service/internal/domain/entities"
	"todo-service/internal/domain/ports"
)

type MySQLTodoRepository struct {
	db *sql.DB
}

func NewMySQLTodoRepository(db *sql.DB) *MySQLTodoRepository {
	return &MySQLTodoRepository{db: db}
}

func (r *MySQLTodoRepository) Create(ctx context.Context, todo *entities.TodoItem) error {
	query := `
		INSERT INTO todos (id, description, due_date, file_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	var fileID interface{}
	if todo.FileID != nil {
		fileID = *todo.FileID
	}

	_, err := r.db.ExecContext(ctx, query,
		todo.ID.String(),
		todo.Description,
		todo.DueDate,
		fileID,
		todo.CreatedAt,
		todo.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create todo: %w", err)
	}

	return nil
}

type MySQLTransactionManager struct {
	db *sql.DB
}

func NewMySQLTransactionManager(db *sql.DB) *MySQLTransactionManager {
	return &MySQLTransactionManager{db: db}
}

func (tm *MySQLTransactionManager) DoInTx(ctx context.Context, fn func(repo ports.TodoRepository) error) error {
	tx, err := tm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	txRepo := &MySQLTxTodoRepository{tx: tx}

	err = fn(txRepo)
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

type MySQLTxTodoRepository struct {
	tx *sql.Tx
}

func (r *MySQLTxTodoRepository) Create(ctx context.Context, todo *entities.TodoItem) error {
	query := `
		INSERT INTO todos (id, description, due_date, file_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	var fileID interface{}
	if todo.FileID != nil {
		fileID = *todo.FileID
	}

	_, err := r.tx.ExecContext(ctx, query,
		todo.ID.String(),
		todo.Description,
		todo.DueDate,
		fileID,
		todo.CreatedAt,
		todo.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create todo: %w", err)
	}

	return nil
}

func (r *MySQLTodoRepository) InitSchema(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS todos (
			id VARCHAR(36) PRIMARY KEY,
			description TEXT NOT NULL,
			due_date TIMESTAMP NOT NULL,
			file_id VARCHAR(36) NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_due_date (due_date),
			INDEX idx_created_at (created_at),
			INDEX idx_file_id (file_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create todos table: %w", err)
	}

	return nil
}
