package core

import (
    "os"
    "testing"
)

func TestStorageEngine(t *testing.T) {
    // Create temp directory for test
    tempDir, err := os.MkdirTemp("", "chronosdb_test")
    if err != nil {
        t.Fatalf("Failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tempDir)
    
    // Create storage engine
    engine, err := NewStorageEngine(tempDir)
    if err != nil {
        t.Fatalf("Failed to create engine: %v", err)
    }
    defer engine.Close()
    
    // Test Put and Get
    key := []byte("test_key")
    value := []byte("test_value")
    
    err = engine.Put(CFNodesCurrent, key, value)
    if err != nil {
        t.Fatalf("Put failed: %v", err)
    }
    
    retrieved, err := engine.Get(CFNodesCurrent, key)
    if err != nil {
        t.Fatalf("Get failed: %v", err)
    }
    
    if string(retrieved) != string(value) {
        t.Errorf("Expected %s, got %s", value, retrieved)
    }
    
    // Test missing key
    missing, err := engine.Get(CFNodesCurrent, []byte("missing"))
    if err != nil {
        t.Fatalf("Get missing failed: %v", err)
    }
    if missing != nil {
        t.Errorf("Expected nil for missing key, got %v", missing)
    }
    
    t.Log("Storage engine test passed!")
}
