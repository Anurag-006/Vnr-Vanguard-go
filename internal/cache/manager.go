package cache

import (
	"time"
	"vnr-vanguard-go/internal/scraper"
)

// The global L1 RAM Cache
var Vault = &MemoryVault{
	store: make(map[string]cacheItem),
}

// GetTieredBatch handles the L1 -> L2 fallback logic
func GetTieredBatch(key string) ([]*scraper.StudentReport, string, bool) {
	// 1. Try L1: RAM Cache (Fastest)
	if data, found := Vault.Get(key); found {
		return data, "memory_cache", true
	}

	// 2. Try L2: Redis Cloud Cache (Fast)
	if data, found := GetRedisBatch(key); found {
		// 🚀 BACKFILL: We found it in Redis, but it was missing from RAM.
		// Let's copy it back into RAM so the next person gets it instantly.
		Vault.Set(key, data, 24*time.Hour)
		
		return data, "redis_cache", true
	}

	// 3. Cache Miss (Slow)
	return nil, "", false
}

// SetTieredBatch saves to BOTH caches simultaneously
func SetTieredBatch(key string, data []*scraper.StudentReport, duration time.Duration) {
	// 1. Save to L1 (RAM)
	Vault.Set(key, data, duration)

	// 2. Save to L2 (Redis)
	SetRedisBatch(key, data, duration)
}