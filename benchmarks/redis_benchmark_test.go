package benchmarks

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"

	"todo-service/internal/config"
	"todo-service/internal/domain/entities"
	"todo-service/internal/infrastructure/streams"
)

func BenchmarkRedisStreamPublish(b *testing.B) {
	publisher := setupRedisStreamPublisher(b)
	defer cleanupRedisTestData(b, publisher)
	ctx := context.Background()

	todos := make([]*entities.TodoItem, b.N)
	for i := 0; i < b.N; i++ {
		todos[i] = entities.NewTodoItem(
			fmt.Sprintf("Benchmark todo for Redis %d", i),
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
}

func BenchmarkRedisStreamPublishWithFiles(b *testing.B) {
	publisher := setupRedisStreamPublisher(b)
	defer cleanupRedisTestData(b, publisher)
	ctx := context.Background()

	todos := make([]*entities.TodoItem, b.N)
	for i := 0; i < b.N; i++ {
		fileID := uuid.New().String()
		todos[i] = entities.NewTodoItem(
			fmt.Sprintf("Benchmark todo with file for Redis %d", i),
			time.Now().Add(24*time.Hour),
			&fileID,
		)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := publisher.PublishTodoCreated(ctx, todos[i])
		if err != nil {
			b.Fatalf("Failed to publish message with file: %v", err)
		}
	}
}

func BenchmarkRedisStreamPublishConcurrent(b *testing.B) {
	publisher := setupRedisStreamPublisher(b)
	defer cleanupRedisTestData(b, publisher)
	ctx := context.Background()

	concurrencyLevels := []int{1, 5, 10, 20, 50, 100}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency%d", concurrency), func(b *testing.B) {
			todos := make([]*entities.TodoItem, b.N)
			for i := 0; i < b.N; i++ {
				todos[i] = entities.NewTodoItem(
					fmt.Sprintf("Concurrent benchmark todo %d", i),
					time.Now().Add(24*time.Hour),
					nil,
				)
			}

			b.ResetTimer()

			var wg sync.WaitGroup
			semaphore := make(chan struct{}, concurrency)

			for i := 0; i < b.N; i++ {
				wg.Add(1)
				go func(index int) {
					defer wg.Done()
					semaphore <- struct{}{}        // Acquire
					defer func() { <-semaphore }() // Release

					err := publisher.PublishTodoCreated(ctx, todos[index])
					if err != nil {
						b.Errorf("Failed to publish message concurrently: %v", err)
					}
				}(i)
			}

			wg.Wait()
		})
	}
}

func BenchmarkRedisStreamPublishWithTimeout(b *testing.B) {
	publisher := setupRedisStreamPublisher(b)
	defer cleanupRedisTestData(b, publisher)

	timeouts := []struct {
		name    string
		timeout time.Duration
	}{
		{"100ms", 100 * time.Millisecond},
		{"500ms", 500 * time.Millisecond},
		{"1s", 1 * time.Second},
		{"5s", 5 * time.Second},
	}

	for _, timeout := range timeouts {
		b.Run(timeout.name, func(b *testing.B) {
			todos := make([]*entities.TodoItem, b.N)
			for i := 0; i < b.N; i++ {
				todos[i] = entities.NewTodoItem(
					fmt.Sprintf("Timeout benchmark todo %d", i),
					time.Now().Add(24*time.Hour),
					nil,
				)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), timeout.timeout)

				err := publisher.PublishTodoCreated(ctx, todos[i])
				cancel()

				if err != nil {
					b.Fatalf("Failed to publish message with timeout %v: %v", timeout.timeout, err)
				}
			}
		})
	}
}

func setupRedisStreamPublisher(b *testing.B) *streams.RedisStreamPublisher {
	cfg := config.Load()

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		b.Fatalf("Failed to connect to Redis: %v", err)
	}

	streamName := "benchmark-todo-events"

	cleanupRedisTestDataWithClient(b, client)

	return streams.NewRedisStreamPublisher(client, streamName)
}

func cleanupRedisTestData(b *testing.B, publisher *streams.RedisStreamPublisher) {
	cfg := config.Load()

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer client.Close()

	cleanupRedisTestDataWithClient(b, client)
}

func cleanupRedisTestDataWithClient(b *testing.B, client *redis.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	streamPatterns := []string{
		"benchmark-*",
		"test-*",
		"Test-*",
		"*benchmark*",
		"*test*",
		"todo-events",
	}

	var keysToDelete []string
	for _, pattern := range streamPatterns {
		keys, err := client.Keys(ctx, pattern).Result()
		if err != nil {
			b.Logf("Warning: Failed to get Redis keys for pattern '%s': %v", pattern, err)
			continue
		}
		keysToDelete = append(keysToDelete, keys...)
	}

	uniqueKeys := make(map[string]bool)
	var finalKeys []string
	for _, key := range keysToDelete {
		if !uniqueKeys[key] {
			uniqueKeys[key] = true
			finalKeys = append(finalKeys, key)
		}
	}

	if len(finalKeys) == 0 {
		return
	}

	batchSize := 100
	for i := 0; i < len(finalKeys); i += batchSize {
		end := i + batchSize
		if end > len(finalKeys) {
			end = len(finalKeys)
		}

		batch := finalKeys[i:end]
		deletedCount, err := client.Del(ctx, batch...).Result()
		if err != nil {
			b.Logf("Warning: Failed to delete Redis keys batch: %v", err)
			continue
		}

		if deletedCount > 0 {
			b.Logf("Cleaned up %d Redis keys: %v", deletedCount, batch)
		}
	}

	hashPatterns := []string{
		"*benchmark*",
		"*test*",
		"*Test*",
	}

	for _, pattern := range hashPatterns {
		keys, err := client.Keys(ctx, pattern).Result()
		if err != nil {
			continue
		}

		if len(keys) > 0 {
			deletedCount, err := client.Del(ctx, keys...).Result()
			if err != nil {
				b.Logf("Warning: Failed to delete Redis hash keys: %v", err)
				continue
			}

			if deletedCount > 0 {
				b.Logf("Cleaned up %d Redis hash keys with pattern '%s'", deletedCount, pattern)
			}
		}
	}

	if isTestEnvironment() {
		err := client.FlushDB(ctx).Err()
		if err != nil {
			b.Logf("Warning: Failed to flush Redis test DB: %v", err)
		} else {
			b.Logf("Flushed Redis test database")
		}
	}
}

func isTestEnvironment() bool {
	cfg := config.Load()
	return cfg.Redis.Host == "localhost" || cfg.Redis.Host == "127.0.0.1" || cfg.Redis.DB != 0
}
