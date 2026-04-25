package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

type Result struct {
	URL   string
	Alive bool
}

func main() {
	dir := flag.String("dir", ".", "Directory to scan for markdown files")
	workers := flag.Int("workers", 10, "Number of concurrent workers")
	timeout := flag.Duration("timeout", 5*time.Second, "HTTP client timeout")
	verbose := flag.Bool("verbose", false, "Print dead URLs to stdout")
	flag.Parse()

	urlRegex := regexp.MustCompile(`https?://[^\s"'<>]+`)
	jobs := make(chan string)
	results := make(chan Result)

	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go worker(jobs, results, &wg, *timeout)
	}

	// Feed jobs
	go func() {
		seen := make(map[string]bool)
		err := filepath.Walk(*dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && filepath.Ext(path) == ".md" {
				content, err := os.ReadFile(path)
				if err == nil {
					matches := urlRegex.FindAllString(string(content), -1)
					for _, match := range matches {
						clean := cleanURL(match)
						// Deduplicate before queuing
						if !seen[clean] {
							seen[clean] = true
							jobs <- clean
						}
					}
				}
			}
			return nil
		})
		if err != nil {
			fmt.Printf("Error walking directory: %v\n", err)
		}
		close(jobs) // Signal workers to stop
	}()

	// Wait for workers to finish and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	aliveCount := 0
	deadCount := 0

	// Process results
	for res := range results {
		if res.Alive {
			aliveCount++
		} else {
			deadCount++
			if *verbose {
				fmt.Printf("DEAD: %s\n", res.URL)
			}
		}
	}

	fmt.Println("--- Summary ---")
	fmt.Printf("Total Unique URLs: %d\n", aliveCount+deadCount)
	fmt.Printf("Alive: %d\n", aliveCount)
	fmt.Printf("Dead:  %d\n", deadCount)
}

// cleanURL removes trailing punctuation
func cleanURL(u string) string {
	for len(u) > 0 {
		lastChar := u[len(u)-1]
		if lastChar == ')' || lastChar == ']' || lastChar == '.' || lastChar == ',' {
			u = u[:len(u)-1]
		} else {
			break
		}
	}
	return u
}

// worker validates URLs
func worker(jobs <-chan string, results chan<- Result, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	client := &http.Client{
		Timeout: timeout,
	}

	for url := range jobs {
		req, err := http.NewRequest(http.MethodHead, url, nil)
		if err != nil {
			results <- Result{URL: url, Alive: false}
			continue
		}

		resp, err := client.Do(req)
		
		// Fallback to GET if HEAD is not allowed
		if err == nil && resp.StatusCode == http.StatusMethodNotAllowed {
			resp.Body.Close()
			req, err = http.NewRequest(http.MethodGet, url, nil)
			if err == nil {
				resp, err = client.Do(req)
			}
		}

		if err != nil {
			results <- Result{URL: url, Alive: false}
			continue
		}

		alive := resp.StatusCode >= 200 && resp.StatusCode < 400
		resp.Body.Close()

		results <- Result{URL: url, Alive: alive}
	}
}
