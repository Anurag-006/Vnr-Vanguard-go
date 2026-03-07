package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// rateVisitor tracks how many times an IP has hit the scraper
type rateVisitor struct {
	requests int
	lastSeen time.Time
}

var (
	visitorsMu sync.Mutex
	visitors   = make(map[string]*rateVisitor)
)

func init() {
	go cleanupVisitors()
}

func cleanupVisitors() {
	for {
		time.Sleep(1 * time.Hour) // Run every hour
		visitorsMu.Lock()
		for ip, v := range visitors {
			if time.Since(v.lastSeen) > 1*time.Hour {
				delete(visitors, ip)
			}
		}
		visitorsMu.Unlock()
	}
}

// RateLimiter restricts heavy endpoints (like live scraping) to 'limit' requests per 'window'
func RateLimiter(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		
		visitorsMu.Lock()
		v, exists := visitors[ip]
		
		// If new IP, or their time window has expired, reset them
		if !exists || time.Since(v.lastSeen) > window {
			visitors[ip] = &rateVisitor{requests: 1, lastSeen: time.Now()}
			visitorsMu.Unlock()
			c.Next()
			return
		}

		// If they exceed the limit within the time window, block them
		if v.requests >= limit {
			visitorsMu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please wait 60 seconds before triggering another live scrape.",
			})
			return
		}

		// Otherwise, increment their request count and let them through
		v.requests++
		visitorsMu.Unlock()
		c.Next()
	}
}