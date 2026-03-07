package cache

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"
	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
	"vnr-vanguard-go/internal/scraper"
)

var (
	Rdb *redis.Client
	Ctx = context.Background()
)

// InitRedis connects to Upstash using the URL in your .env
func InitRedis() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Println("⚠️ REDIS_URL not found. Bypassing Redis.")
		return
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("❌ Failed to parse Redis URL: %v", err)
	}

	Rdb = redis.NewClient(opt)

	// Ping to verify connection
	if err := Rdb.Ping(Ctx).Err(); err != nil {
		log.Fatalf("❌ Failed to connect to Redis: %v", err)
	}

	log.Println("✅ Successfully connected to Upstash Redis Vault")
}

// SetBatch saves the entire slice of students as a single compressed JSON string
func SetRedisBatch(key string, data []*scraper.StudentReport, expiration time.Duration) {
	if Rdb == nil {
		return // Failsafe if Redis isn't connected
	}

	// 1. Convert Go Structs to JSON Bytes
	jsonData, err := sonic.Marshal(data)
	if err != nil {
		log.Printf("Failed to marshal batch for Redis: %v\n", err)
		return
	}

	// 2. Save to Redis
	err = Rdb.Set(Ctx, key, jsonData, expiration).Err()
	if err != nil {
		log.Printf("Failed to save to Redis: %v\n", err)
	}
}

// GetBatch retrieves the JSON string from Redis and converts it back to Go Structs
func GetRedisBatch(key string) ([]*scraper.StudentReport, bool) {
	if Rdb == nil {
		return nil, false
	}

	val, err := Rdb.Get(Ctx, key).Result()
	if err == redis.Nil {
		return nil, false // Cache miss
	} else if err != nil {
		log.Printf("Redis GET error: %v\n", err)
		return nil, false
	}

	// Found it! Convert JSON back to Structs
	var batch []*scraper.StudentReport
	err = json.Unmarshal([]byte(val), &batch)
	if err != nil {
		log.Printf("Failed to unmarshal Redis data: %v\n", err)
		return nil, false
	}

	return batch, true
}