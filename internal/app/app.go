package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"

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
	DB              *sql.DB
	RedisClient     *redis.Client
	Logger          *zap.Logger
}

type App struct {
	deps   *Dependencies
	server *http.Server
	logger *zap.Logger
}

func New(cfg *config.Config, logger *zap.Logger) (*App, error) {
	deps, err := initDependencies(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize dependencies: %w", err)
	}

	router := setupRoutes(deps)

	server := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &App{
		deps:   deps,
		server: server,
		logger: logger,
	}, nil
}

func (a *App) Start() error {
	a.logger.Info("Starting server", zap.String("addr", a.server.Addr))

	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	a.logger.Info("Shutting down server...")

	if err := a.server.Shutdown(ctx); err != nil {
		a.logger.Error("Server shutdown error", zap.Error(err))
		return err
	}

	if a.deps.DB != nil {
		if err := a.deps.DB.Close(); err != nil {
			a.logger.Error("Database close error", zap.Error(err))
		}
	}

	if a.deps.RedisClient != nil {
		if err := a.deps.RedisClient.Close(); err != nil {
			a.logger.Error("Redis close error", zap.Error(err))
		}
	}

	a.logger.Info("Server shutdown complete")
	return nil
}

func initDependencies(cfg *config.Config, logger *zap.Logger) (*Dependencies, error) {
	db, err := initMySQL(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MySQL: %w", err)
	}

	redisClient, err := initRedis(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis: %w", err)
	}

	awsSession, err := initAWS(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AWS: %w", err)
	}

	todoRepo := repositories.NewMySQLTodoRepository(db)
	txManager := repositories.NewMySQLTransactionManager(db)
	streamPublisher := streams.NewRedisStreamPublisher(redisClient, "todo-events")

	fileStorage, err := storage.NewS3FileStorage(awsSession, cfg.AWS.S3Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 file storage: %w", err)
	}

	todoUseCase := usecases.NewTodoUseCase(txManager, streamPublisher)
	fileUseCase := usecases.NewFileUseCase(fileStorage)

	todoHandler := handlers.NewTodoHandler(todoUseCase)
	fileHandler := handlers.NewFileHandler(fileUseCase)

	if err := todoRepo.InitSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize todo schema: %w", err)
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
		DB:              db,
		RedisClient:     redisClient,
		Logger:          logger,
	}, nil
}

func initMySQL(cfg *config.Config, logger *zap.Logger) (*sql.DB, error) {
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

	logger.Info("MySQL connection established",
		zap.String("host", cfg.DB.Host),
		zap.String("port", cfg.DB.Port),
		zap.String("database", cfg.DB.Name))

	return db, nil
}

func initRedis(cfg *config.Config, logger *zap.Logger) (*redis.Client, error) {
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

	logger.Info("Redis connection established",
		zap.String("host", cfg.Redis.Host),
		zap.String("port", cfg.Redis.Port),
		zap.Int("db", cfg.Redis.DB))

	return client, nil
}

func initAWS(cfg *config.Config, logger *zap.Logger) (*session.Session, error) {
	awsConfig := &aws.Config{
		Region: aws.String(cfg.AWS.Region),
	}

	if cfg.AWS.Endpoint != "" {
		awsConfig.Endpoint = aws.String(cfg.AWS.Endpoint)
		awsConfig.S3ForcePathStyle = aws.Bool(true)
		awsConfig.Credentials = credentials.NewStaticCredentials("test", "test", "")
		logger.Info("AWS configured for local development", zap.String("endpoint", cfg.AWS.Endpoint))
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, err
	}

	logger.Info("AWS session established", zap.String("region", cfg.AWS.Region))
	return sess, nil
}

type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp int64             `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

func setupRoutes(deps *Dependencies) *gin.Engine {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		services := make(map[string]string)
		overallStatus := "healthy"

		if err := deps.DB.PingContext(ctx); err != nil {
			services["mysql"] = "unhealthy: " + err.Error()
			overallStatus = "unhealthy"
		} else {
			services["mysql"] = "healthy"
		}

		if err := deps.RedisClient.Ping(ctx).Err(); err != nil {
			services["redis"] = "unhealthy: " + err.Error()
			overallStatus = "unhealthy"
		} else {
			services["redis"] = "healthy"
		}

		status := HealthStatus{
			Status:    overallStatus,
			Timestamp: time.Now().Unix(),
			Services:  services,
		}

		if overallStatus == "healthy" {
			c.JSON(http.StatusOK, status)
		} else {
			c.JSON(http.StatusServiceUnavailable, status)
		}
	})

	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
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
