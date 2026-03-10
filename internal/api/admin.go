package api

import (
	"net/http"
	"runtime"
	"fmt"
	"time"
	"github.com/gin-gonic/gin"
	"vnr-vanguard-go/internal/cache"
)

func GetSystemStatus(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	batchesStored := 0
	totalViews := "0"
	uniqueTotal := int64(0)
	uniqueToday := int64(0)
	var trending []string

	if cache.Rdb != nil {
		// Existing cache size check
		if val, err := cache.Rdb.DBSize(cache.Ctx).Result(); err == nil {
			batchesStored = int(val)
		}

		// 📊 FETCH ANALYTICS
		today := time.Now().Format("2006-01-02")
		
		totalViews = cache.Rdb.Get(cache.Ctx, "analytics:total_views").Val()
		uniqueTotal = cache.Rdb.PFCount(cache.Ctx, "analytics:unique_ips_total").Val()
		uniqueToday = cache.Rdb.PFCount(cache.Ctx, "analytics:unique_ips:"+today).Val()

		// Get Top 5 most searched batches
		topSearches, _ := cache.Rdb.ZRevRangeWithScores(cache.Ctx, "analytics:trending_searches", 0, 4).Result()
		for _, search := range topSearches {
			trending = append(trending, fmt.Sprintf("%s (%v searches)", search.Member.(string), search.Score))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "online",
		"analytics": gin.H{
			"total_page_views":    totalViews,
			"unique_visitors":     uniqueTotal,
			"visitors_today":      uniqueToday,
			"trending_searches":   trending,
		},
		"cache": gin.H{
			"batches_stored": batchesStored,
			"exams_cached":   len(systemExams),
		},
		"memory": gin.H{
			"active_ram_mb": m.Alloc / 1024 / 1024,
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