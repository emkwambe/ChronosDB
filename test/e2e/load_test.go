package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "sync"
    "time"
)

type LoadTestResult struct {
    TotalRequests   int
    SuccessCount    int
    ErrorCount      int
    TotalDuration   time.Duration
    AvgLatency      time.Duration
    MinLatency      time.Duration
    MaxLatency      time.Duration
    RequestsPerSec  float64
}

func runLoadTest(concurrency int, totalRequests int) LoadTestResult {
    start := time.Now()
    results := make(chan time.Duration, totalRequests)
    var wg sync.WaitGroup
    
    // Create worker pool
    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go worker(totalRequests/concurrency, results, &wg)
    }
    
    wg.Wait()
    close(results)
    
    duration := time.Since(start)
    
    // Calculate statistics
    var totalLatency time.Duration
    minLatency := time.Hour
    maxLatency := time.Duration(0)
    successCount := 0
    
    for latency := range results {
        if latency > 0 {
            successCount++
            totalLatency += latency
            if latency < minLatency {
                minLatency = latency
            }
            if latency > maxLatency {
                maxLatency = latency
            }
        }
    }
    
    return LoadTestResult{
        TotalRequests:  totalRequests,
        SuccessCount:   successCount,
        ErrorCount:     totalRequests - successCount,
        TotalDuration:  duration,
        AvgLatency:     totalLatency / time.Duration(successCount),
        MinLatency:     minLatency,
        MaxLatency:     maxLatency,
        RequestsPerSec: float64(successCount) / duration.Seconds(),
    }
}

func worker(requests int, results chan time.Duration, wg *sync.WaitGroup) {
    defer wg.Done()
    
    client := &http.Client{Timeout: 10 * time.Second}
    
    for i := 0; i < requests; i++ {
        start := time.Now()
        query := fmt.Sprintf(`{"query": "MATCH (n) RETURN n LIMIT 10"}`)
        
        req, _ := http.NewRequest("POST", "http://localhost:8080/v1/db/test/query", bytes.NewBufferString(query))
        req.Header.Set("Content-Type", "application/json")
        
        resp, err := client.Do(req)
        latency := time.Since(start)
        
        if err == nil && resp.StatusCode == 200 {
            results <- latency
            resp.Body.Close()
        } else {
            results <- -1
        }
        
        // Small delay to avoid overwhelming
        time.Sleep(10 * time.Millisecond)
    }
}

func main() {
    fmt.Println("=== ChronosDB Load Test ===")
    fmt.Println()
    
    // Test different concurrency levels
    concurrencyLevels := []int{1, 5, 10, 20, 50}
    
    for _, c := range concurrencyLevels {
        fmt.Printf("Testing with %d concurrent workers...\n", c)
        result := runLoadTest(c, 100)
        
        fmt.Printf("  Total Requests: %d\n", result.TotalRequests)
        fmt.Printf("  Successful: %d\n", result.SuccessCount)
        fmt.Printf("  Failed: %d\n", result.ErrorCount)
        fmt.Printf("  Success Rate: %.1f%%\n", float64(result.SuccessCount)/float64(result.TotalRequests)*100)
        fmt.Printf("  Avg Latency: %v\n", result.AvgLatency)
        fmt.Printf("  Min Latency: %v\n", result.MinLatency)
        fmt.Printf("  Max Latency: %v\n", result.MaxLatency)
        fmt.Printf("  Requests/sec: %.2f\n", result.RequestsPerSec)
        fmt.Println()
    }
}
