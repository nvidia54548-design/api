package utils

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// CacheClient wraps Redis client with helper methods
type CacheClient struct {
	client *redis.Client
	logger *zap.SugaredLogger
}

var globalCache *CacheClient

func cacheInitTimeout() time.Duration {
	v := os.Getenv("CACHE_INIT_TIMEOUT_MS")
	if v == "" {
		return 800 * time.Millisecond
	}

	ms, err := strconv.Atoi(v)
	if err != nil || ms <= 0 {
		return 800 * time.Millisecond
	}

	return time.Duration(ms) * time.Millisecond
}

// InitCache initializes the Redis cache client
func InitCache(logger *zap.SugaredLogger) error {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		if logger != nil {
			logger.Warn("REDIS_URL not set, caching disabled")
		}
		return nil
	}

	initTimeout := cacheInitTimeout()

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		if logger != nil {
			logger.Errorw("Failed to parse Redis URL", "error", err)
		}
		return err
	}

	if opts.DialTimeout == 0 {
		opts.DialTimeout = initTimeout
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), initTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		if logger != nil {
			logger.Errorw("Failed to connect to Redis", "error", err)
		}
		return err
	}

	globalCache = &CacheClient{
		client: client,
		logger: logger,
	}

	if logger != nil {
		logger.Infow("Redis cache initialized successfully", "init_timeout", initTimeout.String())
	}
	return nil
}

// GetCache returns the global cache client
func GetCache() *CacheClient {
	return globalCache
}

// CloseCache closes the Redis connection
func CloseCache() error {
	if globalCache != nil && globalCache.client != nil {
		return globalCache.client.Close()
	}
	return nil
}

// CacheEnabled returns true if caching is available
func CacheEnabled() bool {
	return globalCache != nil && globalCache.client != nil
}

// Get retrieves a value from cache
func (c *CacheClient) Get(ctx context.Context, key string, dest interface{}) error {
	if c == nil || c.client == nil {
		return redis.Nil
	}

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dest)
}

// Set stores a value in cache with expiration
func (c *CacheClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if c == nil || c.client == nil {
		return nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, data, expiration).Err()
}

// Incr increments a key's value and returns the new value
func (c *CacheClient) Incr(ctx context.Context, key string) (int64, error) {
	if c == nil || c.client == nil {
		return 0, redis.Nil
	}
	return c.client.Incr(ctx, key).Result()
}

// Expire sets expiration on a key
func (c *CacheClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Expire(ctx, key, expiration).Err()
}

// DeletePattern removes all keys matching a pattern
func (c *CacheClient) DeletePattern(ctx context.Context, pattern string) error {
	if c == nil || c.client == nil {
		return nil
	}

	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			c.logger.Warnw("Failed to delete cache key", "key", iter.Val(), "error", err)
		}
	}
	return iter.Err()
}

// Exists checks if a key exists in cache
func (c *CacheClient) Exists(ctx context.Context, key string) bool {
	if c == nil || c.client == nil {
		return false
	}

	result, err := c.client.Exists(ctx, key).Result()
	return err == nil && result > 0
}

// GetOrSet retrieves from cache or sets using the provided function
func (c *CacheClient) GetOrSet(ctx context.Context, key string, dest interface{}, expiration time.Duration, fn func() (interface{}, error)) error {
	// Try to get from cache first
	if err := c.Get(ctx, key, dest); err == nil {
		return nil
	}

	// Call the function to get fresh data
	value, err := fn()
	if err != nil {
		return err
	}

	// Cache the result
	if errSet := c.Set(ctx, key, value, expiration); errSet != nil {
		c.logger.Warnw("Failed to cache value", "key", key, "error", errSet)
	}

	// Copy value to destination
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Cache key builders
const (
	CacheKeyStatistics  = "stats:overall"
	CacheKeyStudent     = "student:"
	CacheKeyStudentList = "students:list:"
	CacheTTLShort       = 1 * time.Minute
	CacheTTLMedium      = 5 * time.Minute
	CacheTTLLong        = 30 * time.Minute
	CacheTTLStatistics  = 10 * time.Minute
)

// StudentCacheKey generates a cache key for a student
func StudentCacheKey(nis string) string {
	return CacheKeyStudent + nis
}

// StudentListCacheKey generates a cache key for student list with filters
func StudentListCacheKey(page, pageSize int, search, kelas, jurusan string) string {
	return CacheKeyStudentList + hash(page, pageSize, search, kelas, jurusan)
}

// hash creates a simple hash for cache key
func hash(values ...interface{}) string {
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	// Simple hash using first 16 chars of JSON
	h := string(data)
	if len(h) > 32 {
		h = h[:32]
	}
	return h
}

// InvalidateStudentCache removes all student-related cache entries
func InvalidateStudentCache(ctx context.Context) error {
	cache := GetCache()
	if cache == nil {
		return nil
	}

	// Delete student list cache
	if err := cache.DeletePattern(ctx, CacheKeyStudentList+"*"); err != nil {
		return err
	}

	return nil
}
