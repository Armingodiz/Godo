package streams

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"todo-service/internal/domain/entities"
)

type RedisStreamPublisher struct {
	client     *redis.Client
	streamName string
}

func NewRedisStreamPublisher(client *redis.Client, streamName string) *RedisStreamPublisher {
	return &RedisStreamPublisher{
		client:     client,
		streamName: streamName,
	}
}

type TodoEvent struct {
	Type      string             `json:"type"`
	TodoID    string             `json:"todo_id"`
	TodoItem  *entities.TodoItem `json:"todo_item,omitempty"`
	Timestamp int64              `json:"timestamp"`
}

func (p *RedisStreamPublisher) PublishTodoCreated(ctx context.Context, todo *entities.TodoItem) error {
	event := TodoEvent{
		Type:      "todo.created",
		TodoID:    todo.ID.String(),
		TodoItem:  todo,
		Timestamp: time.Now().Unix(),
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal todo event: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: p.streamName,
		Values: map[string]interface{}{
			"event_type": event.Type,
			"todo_id":    event.TodoID,
			"data":       string(eventData),
		},
	}

	_, err = p.client.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("failed to publish event to stream: %w", err)
	}

	return nil
}
