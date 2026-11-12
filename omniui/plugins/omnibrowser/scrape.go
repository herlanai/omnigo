package omnibrowser

import (
	"context"
	"encoding/json"
	"sync"
)

// ScrapeResult is detached from the page; failures retain their URL and input order.
type ScrapeResult struct {
	URL   string          `json:"url"`
	Title string          `json:"title,omitempty"`
	Text  string          `json:"text,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
	Error string          `json:"error,omitempty"`
}

// Scrape limits concurrent independent sessions to 32. Cookies are not shared
// across URLs. An empty expression returns extracted visible text.
func Scrape(ctx context.Context, urls []string, concurrency int, options Options, expression string) []ScrapeResult {
	results := make([]ScrapeResult, len(urls))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(len(urls), max(1, min(32, concurrency))) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range jobs {
				result := ScrapeResult{URL: urls[i]}
				page := New(options)
				err := page.Navigate(ctx, urls[i])
				if err == nil {
					var snapshot Snapshot
					snapshot, err = page.Snapshot(ctx)
					if err == nil {
						result.Title = snapshot.Title
						result.Text = snapshot.Text
						if expression != "" {
							result.Value, err = page.Eval(ctx, expression)
						}
					}
				}
				if err != nil {
					result.Error = err.Error()
				}
				_ = page.Close()
				results[i] = result
			}
		}()
	}
	for i := range urls {
		jobs <- i
	}
	close(jobs)
	workers.Wait()
	return results
}
