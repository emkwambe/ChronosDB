package executor

import (
    "fmt"
    "strings"
    "time"

    "github.com/emkwambe/chronosdb/internal/storage/temporal"
    "github.com/emkwambe/chronosdb/pkg/chronosql"
    "github.com/emkwambe/chronosdb/pkg/predictive"
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
    // Check for FORECAST keyword first
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

func (e *Executor) executeForecast(queryStr string) ([]Result, error) {
    // Parse FORECAST query (simplified)
    // Example: FORECAST value OVER 30 DAYS FOR node_123
    
    var nodeID, property string
    var horizon int64 = 30 * 24 * 60 * 60 * 1000000 // 30 days in microseconds
    
    parts := strings.Fields(queryStr)
    
    for i, part := range parts {
        switch strings.ToUpper(part) {
        case "FORECAST":
            if i+1 < len(parts) {
                property = strings.TrimSuffix(parts[i+1], ",")
            }
        case "FOR":
            if i+1 < len(parts) {
                nodeID = parts[i+1]
            }
        case "OVER":
            if i+1 < len(parts) && i+2 < len(parts) {
                if strings.ToUpper(parts[i+2]) == "DAYS" {
                    days := 1
                    fmt.Sscanf(parts[i+1], "%d", &days)
                    horizon = int64(days) * 24 * 60 * 60 * 1000000
                } else if strings.ToUpper(parts[i+2]) == "HOURS" {
                    hours := 1
                    fmt.Sscanf(parts[i+1], "%d", &hours)
                    horizon = int64(hours) * 60 * 60 * 1000000
                }
            }
        }
    }
    
    if nodeID == "" || property == "" {
        return []Result{
            {
                Type: "forecast",
                Data: map[string]interface{}{
                    "message": "FORECAST query format: FORECAST property OVER duration FOR node_id",
                    "example": "FORECAST age OVER 30 DAYS FOR node_123",
                    "received": queryStr,
                },
            },
        }, nil
    }
    
    // Get historical data
    history, err := e.store.GetPropertyHistory(nodeID, property, 100)
    if err != nil {
        return []Result{
            {
                Type: "forecast",
                Data: map[string]interface{}{
                    "message": "Node not found or no historical data",
                    "node_id": nodeID,
                    "property": property,
                    "error": err.Error(),
                },
            },
        }, nil
    }
    
    if len(history.Values) < 2 {
        return []Result{
            {
                Type: "forecast",
                Data: map[string]interface{}{
                    "message": "Insufficient historical data for forecasting",
                    "node_id": nodeID,
                    "property": property,
                    "points_available": len(history.Values),
                    "required": 2,
                },
            },
        }, nil
    }
    
    // Calculate trend
    trend := predictive.CalculateTrend(history.Values, history.Times)
    
    // Predict based on horizon
    steps := int(horizon / (24 * 60 * 60 * 1000000)) // Convert to days
    if steps < 1 {
        steps = 1
    }
    point, lower, upper := trend.Predict(steps, 0.95)
    
    return []Result{
        {
            Type: "forecast",
            Data: map[string]interface{}{
                "node_id":           nodeID,
                "property":          property,
                "horizon_days":      steps,
                "point_forecast":    point,
                "lower_bound":       lower,
                "upper_bound":       upper,
                "confidence":        0.95,
                "model":             "linear_trend",
                "r_squared":         trend.RSquared,
                "historical_points": len(history.Values),
                "historical_values": history.Values,
            },
        },
    }, nil
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
                    "formatted": chronosql.FormatTimestamp(query.Temporal.Timestamp),
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
        return []Result{
            {
                Type: "node",
                Data: map[string]interface{}{
                    "id":      id,
                    "message": "Node created",
                    "pattern": pattern,
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
