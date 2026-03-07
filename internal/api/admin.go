package api

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"vnr-vanguard-go/internal/cache"
)

func GetSystemStatus(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	allocMB := m.Alloc / 1024 / 1024
	sysMB := m.Sys / 1024 / 1024

	// 🚀 THE REDIS SWAP: Ask Redis how many batches it has
	batchesStored := 0
	if cache.Rdb != nil {
		val, err := cache.Rdb.DBSize(cache.Ctx).Result()
		if err == nil {
			batchesStored = int(val)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "online",
		"cache": gin.H{
			"batches_stored": batchesStored,
			"exams_cached":   len(systemExams),
		},
		"memory": gin.H{
			"active_ram_mb": allocMB,
			"total_sys_mb":  sysMB,
			"goroutines":    runtime.NumGoroutine(),
		},
	})
}

func FlushCache(c *gin.Context) {
	// 1. Nuke L1 RAM
	cache.Vault.Flush()

	// 2. Nuke L2 Redis
	if cache.Rdb != nil {
		cache.Rdb.FlushDB(cache.Ctx)
	}

	// 3. Nuke Metadata
	examsMutex.Lock()
	systemExams = nil
	examsMutex.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"message": "L1 RAM and L2 Redis Vaults successfully annihilated.",
	})
}