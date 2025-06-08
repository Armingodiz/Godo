package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"todo-service/internal/usecases"
)

type TodoHandler struct {
	todoUseCase *usecases.TodoUseCase
}

func NewTodoHandler(todoUseCase *usecases.TodoUseCase) *TodoHandler {
	return &TodoHandler{
		todoUseCase: todoUseCase,
	}
}

func (h *TodoHandler) CreateTodo(c *gin.Context) {
	var req usecases.CreateTodoRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	todo, err := h.todoUseCase.CreateTodo(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create todo",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Todo created successfully",
		"data":    todo,
	})
}
