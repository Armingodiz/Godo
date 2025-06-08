package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"

	"todo-service/internal/config"
	"todo-service/internal/domain/ports"
	"todo-service/internal/infrastructure/repositories"
	"todo-service/internal/infrastructure/storage"
	"todo-service/internal/infrastructure/streams"
	"todo-service/internal/interfaces/http/handlers"
	"todo-service/internal/usecases"
)

type Dependencies struct {
	TodoRepo        ports.TodoRepository
	TxManager       ports.TransactionManager
	StreamPublisher ports.StreamPublisher
	FileStorage     ports.FileStorage
	TodoUseCase     *usecases.TodoUseCase
	FileUseCase     *usecases.FileUseCase
	TodoHandler     *handlers.TodoHandler
	FileHandler     *handlers.FileHandler
}

func InitDependencies(cfg *config.Config) (*Dependencies, error) {
	db, err := initMySQL(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MySQL: %w", err)
	}

	redisClient, err := initRedis(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis: %w", err)
	}

	awsSession, err := initAWS(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AWS: %w", err)
	}

	todoRepo := repositories.NewMySQLTodoRepository(db)
	txManager := repositories.NewMySQLTransactionManager(db)
	streamPublisher := streams.NewRedisStreamPublisher(redisClient, "todo-events")
	fileStorage := storage.NewS3FileStorage(awsSession, cfg.AWS.S3Bucket)

	todoUseCase := usecases.NewTodoUseCase(todoRepo, txManager, streamPublisher)
	fileUseCase := usecases.NewFileUseCase(fileStorage)

	todoHandler := handlers.NewTodoHandler(todoUseCase)
	fileHandler := handlers.NewFileHandler(fileUseCase)

	if err := todoRepo.InitSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize todo schema: %w", err)
	}

	if err := fileStorage.EnsureBucket(context.Background()); err != nil {
		log.Printf("Warning: failed to ensure S3 bucket exists: %v", err)
	}

	return &Dependencies{
		TodoRepo:        todoRepo,
		TxManager:       txManager,
		StreamPublisher: streamPublisher,
		FileStorage:     fileStorage,
		TodoUseCase:     todoUseCase,
		FileUseCase:     fileUseCase,
		TodoHandler:     todoHandler,
		FileHandler:     fileHandler,
	}, nil
}

func initMySQL(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	return db, nil
}

func initRedis(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}

func initAWS(cfg *config.Config) (*session.Session, error) {
	awsConfig := &aws.Config{
		Region: aws.String(cfg.AWS.Region),
	}

	if cfg.AWS.Endpoint != "" {
		awsConfig.Endpoint = aws.String(cfg.AWS.Endpoint)
		awsConfig.S3ForcePathStyle = aws.Bool(true)
		awsConfig.Credentials = credentials.NewStaticCredentials("test", "test", "")
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, err
	}

	return sess, nil
}

func setupRoutes(deps *Dependencies) *gin.Engine {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
		})
	})

	v1 := router.Group("/api/v1")
	{
		v1.POST("/todo", deps.TodoHandler.CreateTodo)
		v1.POST("/upload", deps.FileHandler.UploadFile)
	}

	return router
}

func main() {
	cfg := config.Load()

	deps, err := InitDependencies(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize dependencies: %v", err)
	}

	router := setupRoutes(deps)

	addr := ":" + cfg.App.Port
	log.Printf("Starting server on %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
