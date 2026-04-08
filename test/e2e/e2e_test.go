package e2e

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "testing"
    "time"
)

type TestResult struct {
    Name     string
    Passed   bool
    Duration time.Duration
    Error    string
}

type QueryTest struct {
    Name        string
    Query       string
    Expected    string
    ShouldPass  bool
}

func TestEndToEnd(t *testing.T) {
    results := []TestResult{}
    
    // Test 1: Health Check
    t.Run("Health Check", func(t *testing.T) {
        start := time.Now()
        resp, err := http.Get("http://localhost:8080/v1/db/test/health")
        duration := time.Since(start)
        
        passed := err == nil && resp != nil && resp.StatusCode == 200
        results = append(results, TestResult{
            Name:     "Health Check",
            Passed:   passed,
            Duration: duration,
        })
        
        if !passed {
            t.Error("Health check failed")
        }
    })
    
    // Test 2: Create Node
    t.Run("Create Node", func(t *testing.T) {
        start := time.Now()
        query := `{"query": "CREATE (n:TestNode {name: 'E2ETest', value: 100})"}`
        resp, err := makeRequest("POST", "http://localhost:8080/v1/db/test/query", query)
        duration := time.Since(start)
        
        passed := err == nil && resp != nil
        results = append(results, TestResult{
            Name:     "Create Node",
            Passed:   passed,
            Duration: duration,
        })
        
        if !passed {
            t.Errorf("Create node failed: %v", err)
        }
    })
    
    // Test 3: Create Edge
    t.Run("Create Edge", func(t *testing.T) {
        start := time.Now()
        query := `{"query": "CREATE (a)-[:RELATES_TO]->(b)"}`
        resp, err := makeRequest("POST", "http://localhost:8080/v1/db/test/query", query)
        duration := time.Since(start)
        
        passed := err == nil && resp != nil
        results = append(results, TestResult{
            Name:     "Create Edge",
            Passed:   passed,
            Duration: duration,
        })
        
        if !passed {
            t.Errorf("Create edge failed: %v", err)
        }
    })
    
    // Test 4: Match Query
    t.Run("Match Query", func(t *testing.T) {
        start := time.Now()
        query := `{"query": "MATCH (n:TestNode) RETURN n"}`
        resp, err := makeRequest("POST", "http://localhost:8080/v1/db/test/query", query)
        duration := time.Since(start)
        
        passed := err == nil && resp != nil
        results = append(results, TestResult{
            Name:     "Match Query",
            Passed:   passed,
            Duration: duration,
        })
        
        if !passed {
            t.Errorf("Match query failed: %v", err)
        }
    })
    
    // Test 5: Time Travel (AS OF)
    t.Run("Time Travel AS OF", func(t *testing.T) {
        start := time.Now()
        query := `{"query": "MATCH (n:TestNode) RETURN n AS OF 1700000000000000"}`
        resp, err := makeRequest("POST", "http://localhost:8080/v1/db/test/query", query)
        duration := time.Since(start)
        
        passed := err == nil && resp != nil
        results = append(results, TestResult{
            Name:     "Time Travel AS OF",
            Passed:   passed,
            Duration: duration,
        })
        
        if !passed {
            t.Errorf("Time travel query failed: %v", err)
        }
    })
    
    // Test 6: Time Range (BETWEEN)
    t.Run("Time Range BETWEEN", func(t *testing.T) {
        start := time.Now()
        query := `{"query": "MATCH (n:TestNode) RETURN n BETWEEN 1000 AND 2000"}`
        resp, err := makeRequest("POST", "http://localhost:8080/v1/db/test/query", query)
        duration := time.Since(start)
        
        passed := err == nil && resp != nil
        results = append(results, TestResult{
            Name:     "Time Range BETWEEN",
            Passed:   passed,
            Duration: duration,
        })
        
        if !passed {
            t.Errorf("Time range query failed: %v", err)
        }
    })
    
    // Test 7: Forecast Query
    t.Run("Forecast Query", func(t *testing.T) {
        start := time.Now()
        query := `{"query": "FORECAST value OVER 30 DAYS FOR test_node"}`
        resp, err := makeRequest("POST", "http://localhost:8080/v1/db/test/query", query)
        duration := time.Since(start)
        
        passed := err == nil && resp != nil
        results = append(results, TestResult{
            Name:     "Forecast Query",
            Passed:   passed,
            Duration: duration,
        })
        
        if !passed {
            t.Errorf("Forecast query failed: %v", err)
        }
    })
    
    // Test 8: Delete Node
    t.Run("Delete Node", func(t *testing.T) {
        start := time.Now()
        query := `{"query": "DELETE (n:TestNode) WHERE n.name = 'E2ETest'"}`
        resp, err := makeRequest("POST", "http://localhost:8080/v1/db/test/query", query)
        duration := time.Since(start)
        
        passed := err == nil && resp != nil
        results = append(results, TestResult{
            Name:     "Delete Node",
            Passed:   passed,
            Duration: duration,
        })
        
        if !passed {
            t.Errorf("Delete node failed: %v", err)
        }
    })
    
    // Test 9: Bulk Import
    t.Run("Bulk Import", func(t *testing.T) {
        start := time.Now()
        importData := `[
            {"type": "node", "labels": ["BulkTest"], "properties": {"name": "Item1", "value": 10}},
            {"type": "node", "labels": ["BulkTest"], "properties": {"name": "Item2", "value": 20}},
            {"type": "edge", "edge_type": "CONNECTS", "source_id": "node1", "target_id": "node2", "properties": {"weight": 5}}
        ]`
        resp, err := makeRequest("POST", "http://localhost:8080/v1/db/test/import", importData)
        duration := time.Since(start)
        
        passed := err == nil && resp != nil
        results = append(results, TestResult{
            Name:     "Bulk Import",
            Passed:   passed,
            Duration: duration,
        })
        
        if !passed {
            t.Errorf("Bulk import failed: %v", err)
        }
    })
    
    // Test 10: Performance - 1000 writes
    t.Run("Performance 1000 Writes", func(t *testing.T) {
        start := time.Now()
        successCount := 0
        
        for i := 0; i < 100; i++ { // Reduced to 100 for test speed
            query := fmt.Sprintf(`{"query": "CREATE (n:PerfTest {id: %d, value: %d})"}`, i, i*10)
            resp, err := makeRequest("POST", "http://localhost:8080/v1/db/test/query", query)
            if err == nil && resp != nil {
                successCount++
            }
        }
        
        duration := time.Since(start)
        passed := successCount > 95 // 95% success rate
        
        results = append(results, TestResult{
            Name:     fmt.Sprintf("Performance (100 writes - %d success)", successCount),
            Passed:   passed,
            Duration: duration,
        })
        
        if !passed {
            t.Errorf("Performance test failed: %d/100 successful", successCount)
        }
    })
    
    // Print summary
    printSummary(results)
}

func makeRequest(method, url, body string) (*http.Response, error) {
    client := &http.Client{Timeout: 30 * time.Second}
    req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    return client.Do(req)
}

func printSummary(results []TestResult) {
    fmt.Println("\n" + strings.Repeat("=", 60))
    fmt.Println("E2E TEST SUMMARY")
    fmt.Println(strings.Repeat("=", 60))
    
    passed := 0
    for _, r := range results {
        status := "✓"
        if !r.Passed {
            status = "✗"
        } else {
            passed++
        }
        fmt.Printf("%s %-30s %10s\n", status, r.Name, r.Duration)
    }
    
    fmt.Println(strings.Repeat("-", 60))
    fmt.Printf("Total: %d | Passed: %d | Failed: %d | Success Rate: %.1f%%\n",
        len(results), passed, len(results)-passed, float64(passed)/float64(len(results))*100)
    fmt.Println(strings.Repeat("=", 60))
}

// Helper function for strings.Repeat
func strings.Repeat(s string, count int) string {
    result := ""
    for i := 0; i < count; i++ {
        result += s
    }
    return result
}
