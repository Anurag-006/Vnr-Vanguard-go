package scraper

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"io"
	"regexp"
	"encoding/json"
	"time"
	"sync"
	"github.com/PuerkitoBio/goquery"
)

type Subject struct {
	Code   string `json:"code"`
	Title  string `json:"title"`
	Grade  string `json:"grade"`
	Points string `json:"points"`
	Result string `json:"result"`
}

type StudentReport struct {
	Roll    string    `json:"roll"`
	Name    string    `json:"name"`
	SGPA    string    `json:"sgpa"`
	Verdict string    `json:"verdict"`
	Subjects []Subject `json:"subjects"`
}

type RawExam struct {
	ExamId   int    `json:"ExamId"`
	ExamName string `json:"ExamName"`
}

type ExamOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}


var globalSemaphore = make(chan struct{}, 15)
var globalClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		},
	},
}


func FetchIndividualReport(ctx context.Context, rollNo string, examID string) (*StudentReport, error) {
	url := "https://vnrvjietexams.net/EduPrime3Exam/Results/Results"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request for %s: %w", rollNo, err)
	}
	
	q := req.URL.Query()
	q.Add("htno", rollNo)
	q.Add("examId", examID)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Vanguard/1.0")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", "https://vnrvjietexams.net/EduPrime3Exam/Results")

	resp, err := globalClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("network error fetching %s: %w", rollNo, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("college portal returned HTTP %d for %s", resp.StatusCode, rollNo)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML for %s: %w", rollNo, err)
	}

	report := &StudentReport{Roll: rollNo, SGPA: "0.00", Verdict: "WITHHELD", Name: "WITHHELD STUDENT"}

	doc.Find("td").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "Student Name" {
			report.Name = strings.TrimSpace(s.Next().Find("b").Text())
		}
		if text == "SGPA" {
			report.SGPA = strings.TrimSpace(s.Next().Find("b").Text())
		}
		if text == "Result" {
			report.Verdict = strings.TrimSpace(s.Next().Find("b").Text())
		}
	})

doc.Find("tr").Each(func(i int, row *goquery.Selection) {
    cols := row.Find("td")
    // Ensure we are skipping the header and targeting a data row
    if cols.Length() >= 8 && !strings.Contains(cols.Eq(0).Text(), "Sno") {
        // Extract raw text
        grade := strings.TrimSpace(cols.Eq(4).Text())
        points := strings.TrimSpace(cols.Eq(5).Text())
        
        // Validation: If points are empty but grade is "F", points should be "0"
        if points == "" && grade == "F" {
            points = "0"
        }

        sub := Subject{
            Code:   strings.TrimSpace(cols.Eq(1).Text()),
            Title:  strings.TrimSpace(cols.Eq(2).Text()),
            Grade:  grade,
            Points: points, // Ensure this matches index 5 (6th column)
            Result: strings.TrimSpace(cols.Eq(7).Text()),
        }
        report.Subjects = append(report.Subjects, sub)
    }
})

	if report.Name == "" {
        return nil, nil
    }

	return report, nil
}


func FetchBatch(ctx context.Context, rollNumbers []string, examID string) []*StudentReport {
	// This acts as a toll booth. We cap simultaneous outgoing requests to 15.
	// It protects the college portal from being overwhelmed and your server from IP bans.
	const maxConcurrent = 15

	// Create a channel to safely collect results without race conditions
	resultsChan := make(chan *StudentReport, len(rollNumbers))
	var wg sync.WaitGroup

	for _, roll := range rollNumbers {
		wg.Add(1)

		go func(r string) {
			defer wg.Done()

			if ctx.Err() != nil {
				return
			}

			globalSemaphore <- struct{}{}
			
			// Defer releasing the slot so the next Goroutine can enter
			defer func() { <-globalSemaphore }()

			// Execute the scrape
			report, err := FetchIndividualReport(ctx, r, examID)
			
			// If one student 404s or fails, we don't crash the whole batch. 
			// We just skip them and log internally (or silently drop them).
			if err != nil {
				return
			}

			// Only keep valid, non-withheld results
			if report != nil {
				resultsChan <- report
			}

		}(roll)
	}

	// We run the Wait() in a background goroutine. 
	// This allows the main thread to immediately start pulling finished reports out of 
	// the resultsChan below, preventing pipeline deadlocks.
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// 5. Collect all data from the conveyor belt into a standard slice
	var batch []*StudentReport
	for report := range resultsChan {
		batch = append(batch, report)
	}

	return batch
}

func FetchActiveExams(ctx context.Context) ([]ExamOption, error) {
	url := "https://vnrvjietexams.net/EduPrime3Exam/Results"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Vanguard/1.0")

	resp, err := globalClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	// 1. Read the entire HTML page into memory
	htmlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	htmlStr := string(htmlBytes)

	// Safely isolate the JSON array assigned to "var data = [...]"
	regex := regexp.MustCompile(`var data\s*=\s*(\[.*?\]);`)
	match := regex.FindStringSubmatch(htmlStr)

	if len(match) < 2 {
		return nil, fmt.Errorf("failed to locate exam JSON array in HTML")
	}

	jsonText := match[1]

	// 3. Unmarshal the raw JSON
	var rawExams []RawExam
	if err := json.Unmarshal([]byte(jsonText), &rawExams); err != nil {
		return nil, fmt.Errorf("failed to parse exam JSON: %v", err)
	}

	// 4. Filter and map to our clean struct
	var validExams []ExamOption
	for _, e := range rawExams {
		if strings.Contains(strings.ToUpper(e.ExamName), "B.TECH") {
			validExams = append(validExams, ExamOption{
				ID:   fmt.Sprintf("%d", e.ExamId),
				Name: e.ExamName,
			})
		}
	}

	// Fallback if the portal is misbehaving but the regex worked
	if len(validExams) == 0 {
		validExams = append(validExams, ExamOption{ID: "7463", Name: "B.Tech Regular Exams (Fallback)"})
	}

	return validExams, nil
}