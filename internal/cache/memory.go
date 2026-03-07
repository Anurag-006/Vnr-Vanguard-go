package cache

import (
	"sync"
	"time"

	"vnr-vanguard-go/internal/scraper"
)

type cacheItem struct {
	Data      []*scraper.StudentReport
	ExpiresAt int64
}

type MemoryVault struct {
	// RWMutex allows infinite simultaneous readers, but only ONE writer at a time.
	mu    sync.RWMutex
	store map[string]cacheItem
}

// We instantiate ONE single vault when the server boots up.
// var Vault = &MemoryVault{
// 	store: make(map[string]cacheItem),
// }

func (v *MemoryVault) Flush() {
	v.mu.Lock()
	defer v.mu.Unlock()
	// Overwrite the old map with a brand new, empty one.
	// Go's Garbage Collector will automatically free up the old RAM.
	v.store = make(map[string]cacheItem)
}

// GetItemCount returns the number of cached batches
func (v *MemoryVault) GetItemCount() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.store)
}

// Set locks the vault and safely stores the scraped class data.
func (v *MemoryVault) Set(key string, data []*scraper.StudentReport, duration time.Duration) {
	v.mu.Lock()         // 🔒 LOCK WRITER: Nobody else can read or write right now.
	defer v.mu.Unlock() // 🔓 UNLOCK: Guarantees the vault unlocks even if a panic occurs.

	v.store[key] = cacheItem{
		Data:      data,
		ExpiresAt: time.Now().Add(duration).Unix(),
	}
}

// Get safely reads from the vault. 
// Returns the batch data and a boolean (true = Cache Hit, false = Cache Miss).
func (v *MemoryVault) Get(key string) ([]*scraper.StudentReport, bool) {
	v.mu.RLock()         // 📖 LOCK READER: Multiple Goroutines can read at the exact same time.
	defer v.mu.RUnlock() // 📖 UNLOCK READER

	item, exists := v.store[key]
	if !exists {
		return nil, false
	}

	if time.Now().Unix() > item.ExpiresAt {
		return nil, false // Cache expired
	}

	return item.Data, true // Cache Hit!
}