package api

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"vnr-vanguard-go/internal/cache"
	"vnr-vanguard-go/internal/scraper"
	"vnr-vanguard-go/internal/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"
)

type SquadRequest struct {
	RollNumbers string `json:"roll_numbers"`
	ExamID      string `json:"exam_id"`
}

var requestGroup singleflight.Group

func GetSquadBatch(c *gin.Context) {
	var req SquadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.ExamID == "" {
		req.ExamID = "7463"
	}

	// Extract all 10-character alphanumeric sequences using Regex
	re := regexp.MustCompile(`[a-zA-Z0-9]{10}`)
	rawRolls := re.FindAllString(req.RollNumbers, -1)

	if len(rawRolls) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid 10-digit roll numbers found in input."})
		return
	}

	// Deduplicate and uppercase the rolls
	rollMap := make(map[string]bool)
	var validRolls []string
	for _, r := range rawRolls {
		upperRoll := strings.ToUpper(r)
		if !rollMap[upperRoll] {
			rollMap[upperRoll] = true
			validRolls = append(validRolls, upperRoll)
		}
	}

	// Unleash the scraper (capped at 15 concurrent max)
	results := scraper.FetchBatch(c.Request.Context(), validRolls, req.ExamID)

	// Sort
	sort.Slice(results, func(i, j int) bool {
		valI := parseSGPA(results[i].SGPA)
		valJ := parseSGPA(results[j].SGPA)
		if valI == valJ { return results[i].Name < results[j].Name }
		return valI > valJ
	})

	c.JSON(http.StatusOK, gin.H{"leaderboard": results})
}

// ==========================================
// 📈 CLASS STATS API
// ==========================================
func GetStats(c *gin.Context) {
	sectionKey := c.Query("section")
	yearStr := c.Query("year")
	examID := c.Query("exam")

	cacheKey := fmt.Sprintf("%s-%s-%s", strings.ToUpper(sectionKey), yearStr, examID)
	
	// 🚀 THE REDIS SWAP: Pull stats data from Redis
	students, _, found := cache.GetTieredBatch(cacheKey)

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data not found. Load the leaderboard first."})
		return
	}

	var totalValidSGPA float64
	var validStudentCount int
	var passedStudents int
	var withheldCount int

	type SubjectStat struct {
		Pass int `json:"pass"`
		Fail int `json:"fail"`
	}
	subjectData := make(map[string]*SubjectStat)
	gradeCounts := make(map[string]int)

	for _, s := range students {
		if s.Name == "Result Withheld" || s.SGPA == "0.00" || s.SGPA == "" {
			withheldCount++
			continue 
		}

		val := parseSGPA(s.SGPA)
		if val > 0 {
			totalValidSGPA += val
			validStudentCount++
		}

		hasFail := false
		for _, sub := range s.Subjects {
			if strings.Contains(strings.ToUpper(sub.Result), "FAIL") {
				hasFail = true
			}
			gradeCounts[sub.Grade]++
			
			if _, exists := subjectData[sub.Title]; !exists {
				subjectData[sub.Title] = &SubjectStat{}
			}
			if strings.Contains(strings.ToUpper(sub.Result), "PASS") {
				subjectData[sub.Title].Pass++
			} else {
				subjectData[sub.Title].Fail++
			}
		}

		if !hasFail {
			passedStudents++
		}
	}

	avgSgpa := 0.0
	passPercentage := 0.0
	
	if validStudentCount > 0 {
		avgSgpa = totalValidSGPA / float64(validStudentCount)
		passPercentage = (float64(passedStudents) / float64(validStudentCount)) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"avg_sgpa":        fmt.Sprintf("%.2f", avgSgpa),
		"pass_percentage": fmt.Sprintf("%.1f", passPercentage),
		"total_processed": len(students),
		"valid_count":     validStudentCount,
		"withheld_count":  withheldCount,
		"passed":          passedStudents,
		"failed":          validStudentCount - passedStudents,
		"grades":          gradeCounts,
		"subjects":        subjectData,
	})
}

// GetReport handles the /api/v1/report endpoint
func GetReport(c *gin.Context) {
	roll := c.Query("roll")
	examID := c.Query("exam")

	if examID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Exam is required"})
		return
	}

	if roll == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Roll number is required"})
		return
	}

	data, err := scraper.FetchIndividualReport(c.Request.Context(), roll, examID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Scraping failed: " + err.Error()})
		return
	}

	if data == nil || data.Name == "Result Withheld" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Result Withheld or Not Found"})
		return
	}

	c.JSON(http.StatusOK, data)
}


// Helper function to safely parse SGPA strings into floats for sorting
func parseSGPA(sgpaStr string) float64 {
	// Clean the string just in case
	if sgpaStr == "" || sgpaStr == "0.00" {
        return -1.0
    }
	cleanStr := strings.TrimSpace(strings.ReplaceAll(sgpaStr, ":", ""))
	
	val, err := strconv.ParseFloat(cleanStr, 64)
	if err != nil {
		// If it's "Withheld", "N/A", or a parsing error, sink it to the bottom
		return -1.0
	}
	return val
}

func GetClassBatch(c *gin.Context) {
	sectionKey := c.Query("section")
	yearStr := c.Query("year")
	examID := c.Query("exam")

	if examID == "" { examID = "7463" }

	if sectionKey == "" || yearStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing parameters"})
		return
	}

	sectionInfo, exists := utils.SectionsDB[strings.ToUpper(sectionKey)]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Section not found"})
		return
	}


	cacheKey := fmt.Sprintf("%s-%s-%s", strings.ToUpper(sectionKey), yearStr, examID)
	
	// Increment the search counter for this specific batch
	if cache.Rdb != nil {
		cache.Rdb.ZIncrBy(cache.Ctx, "analytics:trending_searches", 1, cacheKey)
	}

	// 1. FAST PATH: Check the Tiered Cache
	cachedData, sourceStr, found := cache.GetTieredBatch(cacheKey)
	if found {
		c.JSON(http.StatusOK, gin.H{
			"meta": gin.H{
				"section":        sectionKey,
				"year":           yearStr,
				"exam_id":        examID,
				"students_found": len(cachedData),
				"source":         sourceStr,
			},
			"leaderboard": cachedData,
		})
		return
	}

	// 🐌 2. SLOW PATH: CACHE MISS WITH SINGLEFLIGHT PROTECTION
	// If 10 people hit this block at the exact same time for the same cacheKey, 
	// the function inside `Do` only executes ONCE.
	resultsInterface, err, shared := requestGroup.Do(cacheKey, func() (interface{}, error) {
		
		// A. Generate Rolls
		rolls, genErr := utils.GenerateRollNumbers(yearStr, sectionInfo)
		if genErr != nil {
			return nil, genErr
		}

		// B. Unleash Scraper (Happens only ONCE)
		results := scraper.FetchBatch(context.Background(), rolls, examID)
	expectedCount := len(rolls)
actualCount := len(results)

		// C. Sort
		sort.Slice(results, func(i, j int) bool {
			valI := parseSGPA(results[i].SGPA)
			valJ := parseSGPA(results[j].SGPA)
			if valI == valJ { return results[i].Name < results[j].Name }
			return valI > valJ 
		})

		// D. Save to L1 & L2 Cache
if actualCount > 0 && float64(actualCount) >= float64(expectedCount)*0.9 {
    cache.SetTieredBatch(cacheKey, results, 24*time.Hour)
} else {
    fmt.Printf("⚠️ WARNING: Partial scrape detected (%d/%d). Bypassing cache saving.\n", actualCount, expectedCount)
}

		return results, nil
	})

	// Handle errors from the singleflight execution
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert the generic interface{} back to our specific struct slice
	finalResults := resultsInterface.([]*scraper.StudentReport)

	// Determine the source based on whether they were the "driver" or a "passenger"
	sourceMeta := "live_scrape"
	if shared {
		sourceMeta = "live_scrape_shared" // Tells you if Singleflight protected you!
	}

	c.JSON(http.StatusOK, gin.H{
		"meta": gin.H{
			"section":        sectionKey,
			"year":           yearStr,
			"exam_id":        examID,
			"students_found": len(finalResults),
			"source":         sourceMeta,
		},
		"leaderboard": finalResults,
	})
}

// GetSections returns the dynamically available branches from our constants database
func GetSections(c *gin.Context) {
	// Pre-allocate a slice with the exact size of our map
	sections := make([]string, 0, len(utils.SectionsDB))

	for key := range utils.SectionsDB {
		sections = append(sections, key)
	}

	// Go maps iterate in a random order. We must sort alphabetically for the UI.
	sort.Strings(sections)

	c.JSON(http.StatusOK, gin.H{
		"sections": sections,
	})
}

var (
	systemExams   []scraper.ExamOption
	examsMutex    sync.RWMutex
	examsExpireAt time.Time
)

func GetExams(c *gin.Context) {
	examsMutex.RLock() // Lock for reading (allows infinite simultaneous readers)
	
	if len(systemExams) > 0 && time.Now().Before(examsExpireAt) {
		// Serve directly from RAM
		c.JSON(http.StatusOK, gin.H{
			"exams": systemExams,
			"meta":  gin.H{"source": "memory_cache"},
		})
		examsMutex.RUnlock() // Always unlock!
		return
	}
	examsMutex.RUnlock() // Unlock before we go fetch new data

	// 2. 🐌 Slow Path: Live Scrape (Cache Miss or Expired)
	freshExams, err := scraper.FetchActiveExams(c.Request.Context())
	
	if err != nil {
		// If the college portal is down, gracefully fallback to the default exam
		c.JSON(http.StatusOK, gin.H{
			"exams": []scraper.ExamOption{{ID: "7463", Name: "B.Tech Regular Exams (Fallback)"}},
			"meta":  gin.H{"source": "fallback_error"},
		})
		return
	}

	examsMutex.Lock() // Lock for writing (blocks everyone else for 0.001ms)
	systemExams = freshExams
	examsExpireAt = time.Now().Add(24 * time.Hour)
	examsMutex.Unlock()

	// 4. Serve the fresh data
	c.JSON(http.StatusOK, gin.H{
		"exams": systemExams,
		"meta":  gin.H{"source": "live_scrape"},
	})
}

// Add this new handler to handlers.go
func TrackView(c *gin.Context) {
	if cache.Rdb != nil {
		ip := c.ClientIP()
		today := time.Now().Format("2006-01-02") // e.g., "2026-03-11"

		// 1. Increment total views
		cache.Rdb.Incr(cache.Ctx, "analytics:total_views")
		
		// 2. Track unique IP for all-time
		cache.Rdb.PFAdd(cache.Ctx, "analytics:unique_ips_total", ip)
		
		// 3. Track unique IP for today specifically
		cache.Rdb.PFAdd(cache.Ctx, "analytics:unique_ips:"+today, ip)
		
		// Optional: Expire the daily key after 48 hours to save Redis space
		cache.Rdb.Expire(cache.Ctx, "analytics:unique_ips:"+today, 48*time.Hour)
	}
	
	c.JSON(http.StatusOK, gin.H{"status": "tracked"})
}