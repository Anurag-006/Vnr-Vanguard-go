package main

import (
	"log"
	"os"
	"time"

	"vnr-vanguard-go/internal/api"
	"vnr-vanguard-go/internal/cache"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on system environment variables.")
	}

	go cache.InitRedis()

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "supersecretfallback"
	}

	if os.Getenv("GIN_MODE") == "release" {
	    gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// Static Files
	router.Static("/css", "./static/css")
	router.Static("/js", "./static/js")
	router.StaticFile("/", "./static/dashboard.html")
	router.StaticFile("/report", "./static/report_card.html")
	router.StaticFile("/admin", "./static/admin.html")
	router.StaticFile("/friends", "./static/friends.html")
	router.StaticFile("/stats", "./static/stats.html")

	// --- Public API Group ---
	v1 := router.Group("/api/v1")
	{
		v1.GET("/report", api.GetReport)
		v1.GET("/sections", api.GetSections)
		v1.GET("/exams", api.GetExams)
		v1.GET("/stats", api.GetStats) // Fast metadata read
		v1.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{"status": "awake"}) })

		// --- Heavy Scraper Group (Rate Limited) ---
		// We move /class and /squad here. Do NOT register them in v1 above.
		heavy := v1.Group("/")
		heavy.Use(api.RateLimiter(5, time.Minute))
		{
			heavy.GET("/class", api.GetClassBatch)
			heavy.POST("/squad", api.GetSquadBatch)
		}
	}

	// --- Secure Admin Group ---
	adminGroup := router.Group("/api/admin", gin.BasicAuth(gin.Accounts{
		"admin": adminPassword,
	}))
	{
		adminGroup.GET("/status", api.GetSystemStatus)
		adminGroup.POST("/flush", api.FlushCache)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}