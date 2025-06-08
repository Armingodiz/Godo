package entities

import (
	"time"

	"github.com/google/uuid"
)

type TodoItem struct {
	ID          uuid.UUID `json:"id"`
	Description string    `json:"description"`
	DueDate     time.Time `json:"due_date"`
	FileID      *string   `json:"file_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewTodoItem(description string, dueDate time.Time, fileID *string) *TodoItem {
	now := time.Now()
	return &TodoItem{
		ID:          uuid.New(),
		Description: description,
		DueDate:     dueDate,
		FileID:      fileID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (t *TodoItem) IsValid() bool {
	return t.Description != "" && t.ID != uuid.Nil
}
