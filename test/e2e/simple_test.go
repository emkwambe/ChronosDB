package e2e

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "testing"
    "time"
)

func TestSimpleE2E(t *testing.T) {
    // Test 1: Health
    t.Run("Health Check", func(t *testing.T) {
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
    })
    
    // Test 2: Create node
    t.Run("Create Node", func(t *testing.T) {
        query := map[string]string{"query": "CREATE (n:Test {name: 'test'})"}
        body, _ := json.Marshal(query)
        
        resp, err := http.Post("http://localhost:8080/v1/db/test/query", "application/json", bytes.NewBuffer(body))
        if err != nil {
            t.Fatalf("Create node failed: %v", err)
        }
        defer resp.Body.Close()
        
        if resp.StatusCode != 200 {
            t.Errorf("Expected status 200, got %d", resp.StatusCode)
        }
    })
    
    // Test 3: Match query
    t.Run("Match Query", func(t *testing.T) {
        query := map[string]string{"query": "MATCH (n:Test) RETURN n"}
        body, _ := json.Marshal(query)
        
        start := time.Now()
        resp, err := http.Post("http://localhost:8080/v1/db/test/query", "application/json", bytes.NewBuffer(body))
        duration := time.Since(start)
        
        if err != nil {
            t.Fatalf("Match query failed: %v", err)
        }
        defer resp.Body.Close()
        
        if resp.StatusCode != 200 {
            t.Errorf("Expected status 200, got %d", resp.StatusCode)
        }
        
        fmt.Printf("Match query took: %v\n", duration)
    })
    
    // Test 4: Time travel
    t.Run("Time Travel", func(t *testing.T) {
        query := map[string]string{"query": "MATCH (n:Test) RETURN n AS OF 1700000000000000"}
        body, _ := json.Marshal(query)
        
        resp, err := http.Post("http://localhost:8080/v1/db/test/query", "application/json", bytes.NewBuffer(body))
        if err != nil {
            t.Fatalf("Time travel failed: %v", err)
        }
        defer resp.Body.Close()
        
        if resp.StatusCode != 200 {
            t.Errorf("Expected status 200, got %d", resp.StatusCode)
        }
    })
    
    // Test 5: Forecast
    t.Run("Forecast", func(t *testing.T) {
        query := map[string]string{"query": "FORECAST test OVER 30 DAYS FOR test_node"}
        body, _ := json.Marshal(query)
        
        resp, err := http.Post("http://localhost:8080/v1/db/test/query", "application/json", bytes.NewBuffer(body))
        if err != nil {
            t.Fatalf("Forecast failed: %v", err)
        }
        defer resp.Body.Close()
        
        if resp.StatusCode != 200 {
            t.Errorf("Expected status 200, got %d", resp.StatusCode)
        }
    })
}
