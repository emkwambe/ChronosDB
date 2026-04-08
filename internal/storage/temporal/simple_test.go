package temporal

import (
    "os"
    "testing"
)

func TestSimpleCreateNode(t *testing.T) {
    tempDir, err := os.MkdirTemp("", "simple_test")
    if err != nil {
        t.Fatalf("Failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tempDir)
    
    store, err := NewTemporalStore(tempDir)
    if err != nil {
        t.Fatalf("Failed to create store: %v", err)
    }
    defer store.Close()
    
    // Simple create
    err = store.CreateNode("node1", []string{"Person"}, map[string]interface{}{
        "name": "Alice",
    }, 1000, 0)
    if err != nil {
        t.Fatalf("CreateNode failed: %v", err)
    }
    
    t.Log("Simple create passed!")
}
