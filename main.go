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
	dirFlag := flag.String("dir", ".", "")
	workersFlag := flag.Int("workers", 10, "")
	flag.Parse()

	urlRegex := regexp.MustCompile(`https?://[^\s"'<>]+`)
	jobs := make(chan string)
	results := make(chan Result)
	processed := make(map[string]bool)

	var wg sync.WaitGroup

	for i := 0; i < *workersFlag; i++ {
		wg.Add(1)
		go worker(jobs, results, &wg)
	}

	go func() {
		err := filepath.Walk(*dirFlag, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && filepath.Ext(path) == ".md" {
				content, err := os.ReadFile(path)
				if err == nil {
					matches := urlRegex.FindAllString(string(content), -1)
					for _, match := range matches {
						clean := cleanURL(match)
						jobs <- clean
					}
				}
			}
			return nil
		})
		if err != nil {
			fmt.Printf("Error walking directory: %v\n", err)
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	aliveCount := 0
	deadCount := 0

	for res := range results {
		if !processed[res.URL] {
			processed[res.URL] = true
			if res.Alive {
				aliveCount++
			} else {
				deadCount++
			}
		}
	}

	fmt.Println("--- Summary ---")
	fmt.Printf("Total Unique URLs: %d\n", aliveCount+deadCount)
	fmt.Printf("Alive: %d\n", aliveCount)
	fmt.Printf("Dead:  %d\n", deadCount)
}

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

func worker(jobs <-chan string, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for url := range jobs {
		req, err := http.NewRequest(http.MethodHead, url, nil)
		if err != nil {
			results <- Result{URL: url, Alive: false}
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			results <- Result{URL: url, Alive: false}
			continue
		}

		alive := resp.StatusCode == http.StatusOK
		resp.Body.Close()

		results <- Result{URL: url, Alive: alive}
	}
}
