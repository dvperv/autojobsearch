package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RedisClient обертка над redis.Client
type RedisClient struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisClient создает нового Redis клиента
func NewRedisClient(addr, password string, db int, logger *zap.Logger) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,

		// Настройки пула
		PoolSize:     10,
		MinIdleConns: 2,
		PoolTimeout:  30 * time.Second,
		IdleTimeout:  5 * time.Minute,

		// Таймауты
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Проверка соединения
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("Redis connection established",
		zap.String("addr", addr),
		zap.Int("db", db))

	return &RedisClient{
		client: client,
		logger: logger,
	}, nil
}

// Close закрывает соединение с Redis
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// Set сохраняет значение
func (r *RedisClient) Set(ctx context.Context, key, value string) error {
	return r.client.Set(ctx, key, value, 0).Err()
}

// SetWithExpiry сохраняет значение с TTL
func (r *RedisClient) SetWithExpiry(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

// Get получает значение
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// GetInt получает значение как int
func (r *RedisClient) GetInt(ctx context.Context, key string) (int, error) {
	val, err := r.client.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// Increment увеличивает значение на 1
func (r *RedisClient) Increment(ctx context.Context, key string) error {
	return r.client.Incr(ctx, key).Err()
}

// Expire устанавливает TTL для ключа
func (r *RedisClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, key, ttl).Err()
}

// TTL получает оставшееся время жизни ключа
func (r *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// Delete удаляет ключ
func (r *RedisClient) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// Exists проверяет существование ключа
func (r *RedisClient) Exists(ctx context.Context, key string) (bool, error) {
	result, err := r.client.Exists(ctx, key).Result()
	return result > 0, err
}

// HSet сохраняет значение в hash
func (r *RedisClient) HSet(ctx context.Context, key, field string, value interface{}) error {
	return r.client.HSet(ctx, key, field, value).Err()
}

// HGet получает значение из hash
func (r *RedisClient) HGet(ctx context.Context, key, field string) (string, error) {
	return r.client.HGet(ctx, key, field).Result()
}

// HGetAll получает все значения из hash
func (r *RedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

// SAdd добавляет значение в set
func (r *RedisClient) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SAdd(ctx, key, members...).Err()
}

// SIsMember проверяет, является ли значение членом set
func (r *RedisClient) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return r.client.SIsMember(ctx, key, member).Result()
}

// ZAdd добавляет значение в sorted set
func (r *RedisClient) ZAdd(ctx context.Context, key string, members ...*redis.Z) error {
	return r.client.ZAdd(ctx, key, members...).Err()
}

// ZRange получает диапазон значений из sorted set
func (r *RedisClient) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.ZRange(ctx, key, start, stop).Result()
}

// LPush добавляет значение в начало списка
func (r *RedisClient) LPush(ctx context.Context, key string, values ...interface{}) error {
	return r.client.LPush(ctx, key, values...).Err()
}

// RPop удаляет и возвращает последний элемент списка
func (r *RedisClient) RPop(ctx context.Context, key string) (string, error) {
	return r.client.RPop(ctx, key).Result()
}

// Publish публикует сообщение в канал
func (r *RedisClient) Publish(ctx context.Context, channel string, message interface{}) error {
	return r.client.Publish(ctx, channel, message).Err()
}

// Subscribe подписывается на канал
func (r *RedisClient) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return r.client.Subscribe(ctx, channel)
}

// Pipeline создает pipeline
func (r *RedisClient) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

// HealthCheck проверка здоровья Redis
func (r *RedisClient) HealthCheck(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// RateLimit проверка rate limit
func (r *RedisClient) RateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	current, err := r.GetInt(ctx, key)
	if err != nil {
		return false, 0, err
	}

	if current >= limit {
		// Получаем TTL ключа
		ttl, err := r.TTL(ctx, key)
		if err != nil {
			return false, 0, err
		}
		return false, ttl, nil
	}

	// Увеличиваем счетчик
	if current == 0 {
		// Первый запрос, устанавливаем TTL
		if err := r.SetWithExpiry(ctx, key, "1", window); err != nil {
			return false, 0, err
		}
	} else {
		if err := r.Increment(ctx, key); err != nil {
			return false, 0, err
		}
	}

	return true, 0, nil
}
