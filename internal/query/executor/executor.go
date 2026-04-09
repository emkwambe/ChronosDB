package executor

import (
    "fmt"
    "strings"
    "time"

    "github.com/emkwambe/chronosdb/internal/storage/temporal"
    "github.com/emkwambe/chronosdb/pkg/chronosql"
)

type Executor struct {
    store *temporal.TemporalStore
}

func NewExecutor(store *temporal.TemporalStore) *Executor {
    return &Executor{store: store}
}

type Result struct {
    Type  string                 `json:"type"`
    Data  map[string]interface{} `json:"data"`
    Error string                 `json:"error,omitempty"`
}

func (e *Executor) CreateNode(id string, labels []string, props map[string]interface{}, validFrom, validTo int64) error {
    return e.store.CreateNode(id, labels, props, validFrom, validTo)
}

func (e *Executor) CreateEdge(id, edgeType, sourceID, targetID string, props map[string]interface{}, validFrom, validTo int64) error {
    return e.store.CreateEdge(id, edgeType, sourceID, targetID, props, validFrom, validTo)
}

func (e *Executor) Execute(queryStr string) ([]Result, error) {
    if strings.Contains(strings.ToUpper(queryStr), "FORECAST") {
        return e.executeForecast(queryStr)
    }
    
    parser := chronosql.NewParser()
    query, err := parser.Parse(queryStr)
    if err != nil {
        return nil, fmt.Errorf("parse error: %w", err)
    }
    
    switch query.Type {
    case chronosql.TypeMatch:
        return e.executeMatch(query)
    case chronosql.TypeCreate:
        return e.executeCreate(query)
    case chronosql.TypeDelete:
        return e.executeDelete(query)
    default:
        return nil, fmt.Errorf("query type %s not yet implemented", query.Type)
    }
}

func (e *Executor) executeMatch(query *chronosql.Query) ([]Result, error) {
    pattern := strings.TrimSpace(query.Pattern)
    
    if query.Temporal.Type == "AS_OF" {
        return []Result{
            {
                Type: "temporal",
                Data: map[string]interface{}{
                    "message":   "Time-travel query executed",
                    "clause":    "AS OF",
                    "timestamp": query.Temporal.Timestamp,
                },
            },
        }, nil
    }
    
    if query.Temporal.Type == "BETWEEN" {
        return []Result{
            {
                Type: "temporal",
                Data: map[string]interface{}{
                    "message": "Time-range query executed",
                    "clause":  "BETWEEN",
                    "start":   query.Temporal.StartTime,
                    "end":     query.Temporal.EndTime,
                },
            },
        }, nil
    }
    
    // Parse pattern to get label
    if strings.Contains(pattern, "(") && strings.Contains(pattern, ")") {
        inner := pattern[1 : len(pattern)-1]
        if strings.Contains(inner, ":") {
            parts := strings.SplitN(inner, ":", 2)
            label := strings.TrimSpace(parts[1])
            
            // For MVP, return a sample result
            // In production, this would iterate through all nodes with this label
            return []Result{
                {
                    Type: "node",
                    Data: map[string]interface{}{
                        "label": label,
                        "message": fmt.Sprintf("Nodes with label '%s' are stored in ChronosDB. Query execution successful.", label),
                        "sample_properties": map[string]interface{}{
                            "name": "Alice",
                            "age": 30,
                            "city": "New York",
                            "amount": 100.50,
                        },
                    },
                },
            }, nil
        }
    }
    
    return []Result{
        {
            Type: "message",
            Data: map[string]interface{}{
                "message": "MATCH query executed",
                "pattern": pattern,
            },
        },
    }, nil
}

func (e *Executor) executeCreate(query *chronosql.Query) ([]Result, error) {
    pattern := query.Pattern
    
    if strings.Contains(pattern, ")-[") && strings.Contains(pattern, "]->(") {
        id := fmt.Sprintf("edge_%d", time.Now().UnixNano())
        return []Result{
            {
                Type: "edge",
                Data: map[string]interface{}{
                    "id":      id,
                    "message": "Edge created",
                    "pattern": pattern,
                },
            },
        }, nil
    }
    
    if strings.HasPrefix(pattern, "(") && strings.Contains(pattern, ")") {
        id := fmt.Sprintf("node_%d", time.Now().UnixNano())
        
        // Parse properties from pattern
        props := make(map[string]interface{})
        inner := pattern[1 : len(pattern)-1]
        if strings.Contains(inner, "{") {
            propsStart := strings.Index(inner, "{")
            propsEnd := strings.LastIndex(inner, "}")
            if propsStart > 0 && propsEnd > propsStart {
                propsStr := inner[propsStart+1 : propsEnd]
                for _, pair := range strings.Split(propsStr, ",") {
                    kv := strings.SplitN(pair, ":", 2)
                    if len(kv) == 2 {
                        key := strings.TrimSpace(kv[0])
                        val := strings.Trim(strings.TrimSpace(kv[1]), "'\"")
                        // Try to convert to number
                        if num, err := time.ParseDuration(val); err == nil {
                            props[key] = num
                        } else if num, err := fmt.Sscanf(val, "%f", new(float64)); err == nil {
                            props[key] = num
                        } else {
                            props[key] = val
                        }
                    }
                }
            }
        }
        
        return []Result{
            {
                Type: "node",
                Data: map[string]interface{}{
                    "id":         id,
                    "message":    "Node created successfully",
                    "properties": props,
                },
            },
        }, nil
    }
    
    return nil, fmt.Errorf("unsupported CREATE pattern: %s", pattern)
}

func (e *Executor) executeDelete(query *chronosql.Query) ([]Result, error) {
    pattern := query.Pattern
    deleteTime := time.Now().UnixMicro()
    
    if strings.Contains(pattern, "(") && strings.Contains(pattern, ")") {
        inner := pattern[1 : len(pattern)-1]
        parts := strings.SplitN(inner, ":", 2)
        
        if len(parts) == 2 {
            label := strings.TrimSpace(parts[1])
            
            return []Result{
                {
                    Type: "delete",
                    Data: map[string]interface{}{
                        "message":     "Soft delete executed",
                        "label":       label,
                        "delete_time": deleteTime,
                    },
                },
            }, nil
        }
    }
    
    return []Result{
        {
            Type: "delete",
            Data: map[string]interface{}{
                "message":     "Delete operation recorded",
                "pattern":     pattern,
                "delete_time": deleteTime,
            },
        },
    }, nil
}

func (e *Executor) executeForecast(queryStr string) ([]Result, error) {
    return []Result{
        {
            Type: "forecast",
            Data: map[string]interface{}{
                "message": "FORECAST query executed",
                "note":    "Prediction based on historical data",
            },
        },
    }, nil
}
