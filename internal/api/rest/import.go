package rest

import (
    "encoding/csv"
    "encoding/json"
    "fmt"
    "strings"
    "time"
)

type ImportRecord struct {
    Type       string                 `json:"type"`
    ID         string                 `json:"id,omitempty"`
    Labels     []string               `json:"labels,omitempty"`
    Properties map[string]interface{} `json:"properties"`
    SourceID   string                 `json:"source_id,omitempty"`
    TargetID   string                 `json:"target_id,omitempty"`
    EdgeType   string                 `json:"edge_type,omitempty"`
    ValidFrom  int64                  `json:"valid_from"`
    ValidTo    int64                  `json:"valid_to,omitempty"`
}

type ImportResult struct {
    TotalRecords   int      `json:"total_records"`
    NodesCreated   int      `json:"nodes_created"`
    EdgesCreated   int      `json:"edges_created"`
    Errors         []string `json:"errors,omitempty"`
    DurationMs     int64    `json:"duration_ms"`
}

func (s *Server) importJSON(data []byte) (*ImportResult, error) {
    var records []ImportRecord
    if err := json.Unmarshal(data, &records); err != nil {
        return nil, fmt.Errorf("failed to parse JSON: %w", err)
    }
    
    return s.processRecords(records)
}

func (s *Server) importCSV(data []byte) (*ImportResult, error) {
    _ = csv.NewReader(strings.NewReader(string(data)))
    return nil, fmt.Errorf("CSV import coming soon")
}

func (s *Server) processRecords(records []ImportRecord) (*ImportResult, error) {
    result := &ImportResult{
        TotalRecords: len(records),
    }
    startTime := time.Now()
    
    for _, record := range records {
        validFrom := record.ValidFrom
        if validFrom == 0 {
            validFrom = time.Now().UnixMicro()
        }
        
        validTo := record.ValidTo
        
        switch record.Type {
        case "node":
            id := record.ID
            if id == "" {
                id = fmt.Sprintf("node_%d", time.Now().UnixNano())
            }
            
            err := s.executor.CreateNode(id, record.Labels, record.Properties, validFrom, validTo)
            if err != nil {
                result.Errors = append(result.Errors, fmt.Sprintf("Failed to create node: %v", err))
                continue
            }
            result.NodesCreated++
            
        case "edge":
            id := record.ID
            if id == "" {
                id = fmt.Sprintf("edge_%d", time.Now().UnixNano())
            }
            
            err := s.executor.CreateEdge(id, record.EdgeType, record.SourceID, record.TargetID, record.Properties, validFrom, validTo)
            if err != nil {
                result.Errors = append(result.Errors, fmt.Sprintf("Failed to create edge: %v", err))
                continue
            }
            result.EdgesCreated++
            
        default:
            result.Errors = append(result.Errors, fmt.Sprintf("Unknown record type: %s", record.Type))
        }
    }
    
    result.DurationMs = time.Since(startTime).Milliseconds()
    return result, nil
}
