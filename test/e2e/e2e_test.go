package e2e

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "testing"
    "time"
)

func TestE2E(t *testing.T) {
    fmt.Println("\n=== ChronosDB E2E Test Suite ===")
    
    passed := 0
    failed := 0
    
    tests := []struct {
        name string
        fn   func(*testing.T)
    }{
        {"Health Check", testHealth},
        {"Create Node", testCreateNode},
        {"Create Edge", testCreateEdge},
        {"Match Query", testMatchQuery},
        {"Time Travel AS OF", testTimeTravel},
        {"Time Range BETWEEN", testTimeRange},
        {"Forecast Query", testForecast},
        {"Delete Node", testDeleteNode},
        {"Bulk Import", testBulkImport},
        {"Performance", testPerformance},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tt.fn(t)
            if !t.Failed() {
                passed++
                fmt.Printf("✓ %s passed\n", tt.name)
            } else {
                failed++
                fmt.Printf("✗ %s failed\n", tt.name)
            }
        })
    }
    
    fmt.Printf("\n=== Results: %d passed, %d failed ===\n", passed, failed)
}

func testHealth(t *testing.T) {
    resp, err := http.Get("http://localhost:8080/v1/db/test/health")
    if err != nil {
        t.Fatalf("Health check failed: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }
    
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    if result["status"] != "healthy" {
        t.Errorf("Expected healthy, got %v", result["status"])
    }
}

func testCreateNode(t *testing.T) {
    query := map[string]string{"query": "CREATE (n:TestNode {name: 'e2e_test', value: 100})"}
    body, _ := json.Marshal(query)
    
    resp, err := http.Post("http://localhost:8080/v1/db/test/query", "application/json", bytes.NewBuffer(body))
    if err != nil {
        t.Fatalf("Create node failed: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }
}

func testCreateEdge(t *testing.T) {
    query := map[string]string{"query": "CREATE (a)-[:RELATES_TO]->(b)"}
    body, _ := json.Marshal(query)
    
    resp, err := http.Post("http://localhost:8080/v1/db/test/query", "application/json", bytes.NewBuffer(body))
    if err != nil {
        t.Fatalf("Create edge failed: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }
}

func testMatchQuery(t *testing.T) {
    query := map[string]string{"query": "MATCH (n:TestNode) RETURN n"}
    body, _ := json.Marshal(query)
    
    resp, err := http.Post("http://localhost:8080/v1/db/test/query", "application/json", bytes.NewBuffer(body))
    if err != nil {
        t.Fatalf("Match query failed: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }
}

func testTimeTravel(t *testing.T) {
    query := map[string]string{"query": "MATCH (n:TestNode) RETURN n AS OF 1700000000000000"}
    body, _ := json.Marshal(query)
    
    resp, err := http.Post("http://localhost:8080/v1/db/test/query", "application/json", bytes.NewBuffer(body))
    if err != nil {
        t.Fatalf("Time travel failed: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }
}

func testTimeRange(t *testing.T) {
    query := map[string]string{"query": "MATCH (n:TestNode) RETURN n BETWEEN 1000 AND 2000"}
    body, _ := json.Marshal(query)
    
    resp, err := http.Post("http://localhost:8080/v1/db/test/query", "application/json", bytes.NewBuffer(body))
    if err != nil {
        t.Fatalf("Time range failed: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }
}

func testForecast(t *testing.T) {
    query := map[string]string{"query": "FORECAST value OVER 30 DAYS FOR test_node"}
    body, _ := json.Marshal(query)
    
    resp, err := http.Post("http://localhost:8080/v1/db/test/query", "application/json", bytes.NewBuffer(body))
    if err != nil {
        t.Fatalf("Forecast failed: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }
}

func testDeleteNode(t *testing.T) {
    query := map[string]string{"query": "DELETE (n:TestNode) WHERE n.name = 'e2e_test'"}
    body, _ := json.Marshal(query)
    
    resp, err := http.Post("http://localhost:8080/v1/db/test/query", "application/json", bytes.NewBuffer(body))
    if err != nil {
        t.Fatalf("Delete node failed: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }
}

func testBulkImport(t *testing.T) {
    importData := `[
        {"type": "node", "labels": ["BulkTest"], "properties": {"name": "Item1", "value": 10}},
        {"type": "node", "labels": ["BulkTest"], "properties": {"name": "Item2", "value": 20}}
    ]`
    
    resp, err := http.Post("http://localhost:8080/v1/db/test/import", "application/json", bytes.NewBufferString(importData))
    if err != nil {
        t.Fatalf("Bulk import failed: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }
}

func testPerformance(t *testing.T) {
    successCount := 0
    start := time.Now()
    
    for i := 0; i < 50; i++ {
        query := map[string]string{"query": fmt.Sprintf("CREATE (n:PerfTest {id: %d})", i)}
        body, _ := json.Marshal(query)
        
        resp, err := http.Post("http://localhost:8080/v1/db/test/query", "application/json", bytes.NewBuffer(body))
        if err == nil && resp.StatusCode == 200 {
            successCount++
            resp.Body.Close()
        }
    }
    
    duration := time.Since(start)
    t.Logf("50 writes took %v (%.2f writes/sec)", duration, float64(successCount)/duration.Seconds())
    
    if successCount < 45 {
        t.Errorf("Performance poor: only %d/50 successful", successCount)
    }
}
