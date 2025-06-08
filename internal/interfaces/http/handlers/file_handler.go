package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"todo-service/internal/usecases"
)

type FileHandler struct {
	fileUseCase *usecases.FileUseCase
}

func NewFileHandler(fileUseCase *usecases.FileUseCase) *FileHandler {
	return &FileHandler{
		fileUseCase: fileUseCase,
	}
}

func (h *FileHandler) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to get file from request",
			"details": err.Error(),
		})
		return
	}
	defer file.Close()

	req := usecases.UploadFileRequest{
		FileName:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Data:        file,
		Size:        header.Size,
	}

	response, err := h.fileUseCase.UploadFile(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to upload file",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "File uploaded successfully",
		"data":    response,
	})
}
