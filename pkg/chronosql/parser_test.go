package chronosql

import (
    "testing"
)

func TestParseASOF(t *testing.T) {
    parser := NewParser()
    
    tests := []struct {
        name     string
        query    string
        expected string
        hasTime   bool
    }{
        {
            name:     "AS OF with numeric timestamp",
            query:    "MATCH (n:Person) RETURN n AS OF 1700000000000000",
            expected: "AS_OF",
            hasTime:  true,
        },
        {
            name:     "AS OF with quoted timestamp",
            query:    "MATCH (n:Person) RETURN n AS OF '2024-01-01'",
            expected: "AS_OF",
            hasTime:  true,
        },
        {
            name:     "No temporal clause",
            query:    "MATCH (n:Person) RETURN n",
            expected: "",
            hasTime:  false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            q, err := parser.Parse(tt.query)
            if err != nil {
                t.Fatalf("Parse failed: %v", err)
            }
            
            if q.Temporal.Type != tt.expected {
                t.Errorf("Expected temporal type %s, got %s", tt.expected, q.Temporal.Type)
            }
            
            if tt.hasTime && q.Temporal.Timestamp == 0 {
                t.Error("Expected timestamp, got 0")
            }
        })
    }
}

func TestParseBETWEEN(t *testing.T) {
    parser := NewParser()
    
    query := "MATCH (n:Person) RETURN n BETWEEN 1000 AND 2000"
    q, err := parser.Parse(query)
    if err != nil {
        t.Fatalf("Parse failed: %v", err)
    }
    
    if q.Temporal.Type != "BETWEEN" {
        t.Errorf("Expected BETWEEN, got %s", q.Temporal.Type)
    }
    
    if q.Temporal.StartTime != 1000 {
        t.Errorf("Expected start 1000, got %d", q.Temporal.StartTime)
    }
    
    if q.Temporal.EndTime != 2000 {
        t.Errorf("Expected end 2000, got %d", q.Temporal.EndTime)
    }
}

func TestParseQueryType(t *testing.T) {
    parser := NewParser()
    
    queries := []string{
        "MATCH (n) RETURN n",
        "CREATE (n:Person {name: 'Alice'})",
        "UPDATE (n:Person) SET n.age = 31",
        "DELETE (n:Person)",
    }
    
    expectedTypes := []QueryType{TypeMatch, TypeCreate, TypeUpdate, TypeDelete}
    
    for i, query := range queries {
        q, err := parser.Parse(query)
        if err != nil {
            t.Fatalf("Parse failed for %s: %v", query, err)
        }
        
        if q.Type != expectedTypes[i] {
            t.Errorf("Expected type %s, got %s", expectedTypes[i], q.Type)
        }
    }
}

func TestTimestampFormats(t *testing.T) {
    parser := NewParser()
    
    // Test various timestamp formats in AS OF
    timestamps := []string{
        "1700000000000000",
        "'2024-01-01'",
        "'2024-01-01T15:04:05Z'",
    }
    
    for _, ts := range timestamps {
        query := "MATCH (n) RETURN n AS OF " + ts
        _, err := parser.Parse(query)
        if err != nil {
            t.Errorf("Failed to parse timestamp %s: %v", ts, err)
        }
    }
}
