package temporal

import (
    "os"
    "testing"
    "time"
)

func TestTemporalStore(t *testing.T) {
    tempDir, err := os.MkdirTemp("", "temporal_test")
    if err != nil {
        t.Fatalf("Failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tempDir)
    
    store, err := NewTemporalStore(tempDir)
    if err != nil {
        t.Fatalf("Failed to create store: %v", err)
    }
    defer store.Close()
    
    now := time.Now().UnixMicro()
    
    // Test 1: Create node
    t.Log("Test 1: Creating node")
    err = store.CreateNode("node1", []string{"Person"}, map[string]interface{}{
        "name": "Alice",
        "age":  30,
    }, now, 0)
    if err != nil {
        t.Fatalf("CreateNode failed: %v", err)
    }
    
    // Test 2: Get node as of now
    t.Log("Test 2: Getting node as of now")
    node, err := store.GetNodeAsOf("node1", now)
    if err != nil {
        t.Fatalf("GetNodeAsOf failed: %v", err)
    }
    if node == nil {
        t.Fatal("Node not found")
    }
    
    name, ok := node.Properties["name"].(string)
    if !ok || name != "Alice" {
        t.Errorf("Expected name Alice, got %v", node.Properties["name"])
    }
    
    // Test 3: Update property
    t.Log("Test 3: Updating property")
    future := now + 1000
    err = store.UpdateNodeProperty("node1", "age", 31, future)
    if err != nil {
        t.Fatalf("UpdateNodeProperty failed: %v", err)
    }
    
    // Test 4: Get node after update (should show age 31)
    t.Log("Test 4: Getting node after update")
    node, err = store.GetNodeAsOf("node1", future+500)
    if err != nil {
        t.Fatalf("GetNodeAsOf (future) failed: %v", err)
    }
    if node == nil {
        t.Fatal("Node not found after update")
    }
    
    age, ok := node.Properties["age"].(float64)
    if !ok {
        t.Errorf("Age not a number, got %T", node.Properties["age"])
    } else if age != 31 {
        t.Errorf("Expected age 31, got %v", age)
    }
    
    // Test 5: Create edge
    t.Log("Test 5: Creating edge")
    err = store.CreateEdge("edge1", "KNOWS", "node1", "node2", map[string]interface{}{
        "since": 2020,
    }, now, 0)
    if err != nil {
        t.Fatalf("CreateEdge failed: %v", err)
    }
    
    // Test 6: Get edge
    t.Log("Test 6: Getting edge")
    edge, err := store.GetEdgeAsOf("edge1", now)
    if err != nil {
        t.Fatalf("GetEdgeAsOf failed: %v", err)
    }
    if edge == nil {
        t.Fatal("Edge not found")
    }
    if edge.Type != "KNOWS" {
        t.Errorf("Expected type KNOWS, got %s", edge.Type)
    }
    
    t.Log("All temporal store tests passed!")
}
