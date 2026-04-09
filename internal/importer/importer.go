package importer

import (
    "encoding/csv"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "strconv"
    "strings"
    "time"

    "github.com/emkwambe/chronosdb/internal/query/executor"
    "github.com/emkwambe/chronosdb/internal/storage/temporal"
)

type DataImporter struct {
    store    *temporal.TemporalStore
    executor *executor.Executor
}

type ImportStats struct {
    TotalRows    int      `json:"total_rows"`
    NodesCreated int      `json:"nodes_created"`
    EdgesCreated int      `json:"edges_created"`
    Errors       []string `json:"errors,omitempty"`
    Duration     string   `json:"duration"`
}

func NewDataImporter(store *temporal.TemporalStore, exec *executor.Executor) *DataImporter {
    return &DataImporter{
        store:    store,
        executor: exec,
    }
}

// ImportCSV imports data from CSV file
func (di *DataImporter) ImportCSV(filePath, nodeLabel, timestampColumn string) (*ImportStats, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("failed to open CSV: %w", err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    headers, err := reader.Read()
    if err != nil {
        return nil, fmt.Errorf("failed to read headers: %w", err)
    }

    stats := &ImportStats{}
    startTime := time.Now()

    rowNum := 1
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            stats.Errors = append(stats.Errors, fmt.Sprintf("Row %d: %v", rowNum, err))
            continue
        }

        // Build properties map
        props := make(map[string]interface{})
        var timestamp int64 = time.Now().UnixMicro()

        for i, header := range headers {
            if i < len(record) && record[i] != "" {
                if header == timestampColumn {
                    if t, err := parseTimestamp(record[i]); err == nil {
                        timestamp = t
                    }
                } else {
                    // Try to convert to number if possible
                    if num, err := strconv.ParseFloat(record[i], 64); err == nil {
                        props[header] = num
                    } else {
                        props[header] = record[i]
                    }
                }
            }
        }

        // Create node
        nodeID := fmt.Sprintf("%s_%d", nodeLabel, rowNum)
        err = di.store.CreateNode(nodeID, []string{nodeLabel}, props, timestamp, 0)
        if err != nil {
            stats.Errors = append(stats.Errors, fmt.Sprintf("Row %d: %v", rowNum, err))
        } else {
            stats.NodesCreated++
        }
        stats.TotalRows++
        rowNum++
    }

    stats.Duration = time.Since(startTime).String()
    return stats, nil
}

// ImportJSON imports data from JSON file
func (di *DataImporter) ImportJSON(filePath, nodeLabel string) (*ImportStats, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("failed to open JSON: %w", err)
    }
    defer file.Close()

    var data []map[string]interface{}
    if err := json.NewDecoder(file).Decode(&data); err != nil {
        return nil, fmt.Errorf("failed to parse JSON: %w", err)
    }

    stats := &ImportStats{}
    startTime := time.Now()

    for i, record := range data {
        timestamp := time.Now().UnixMicro()
        
        // Check for timestamp field
        if ts, ok := record["timestamp"]; ok {
            if t, ok := ts.(float64); ok {
                timestamp = int64(t)
            }
        }

        // Remove timestamp from properties
        delete(record, "timestamp")

        nodeID := fmt.Sprintf("%s_%d", nodeLabel, i+1)
        err = di.store.CreateNode(nodeID, []string{nodeLabel}, record, timestamp, 0)
        if err != nil {
            stats.Errors = append(stats.Errors, fmt.Sprintf("Record %d: %v", i+1, err))
        } else {
            stats.NodesCreated++
        }
        stats.TotalRows++
    }

    stats.Duration = time.Since(startTime).String()
    return stats, nil
}

// ImportSQL imports data from SQL dump (simplified)
func (di *DataImporter) ImportSQL(filePath, nodeLabel string) (*ImportStats, error) {
    content, err := os.ReadFile(filePath)
    if err != nil {
        return nil, fmt.Errorf("failed to read SQL file: %w", err)
    }

    stats := &ImportStats{}
    startTime := time.Now()

    // Parse INSERT statements (simplified)
    lines := strings.Split(string(content), "\n")
    for i, line := range lines {
        if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "INSERT") {
            // Extract values (simplified parsing)
            if strings.Contains(line, "VALUES") {
                parts := strings.SplitN(line, "VALUES", 2)
                if len(parts) == 2 {
                    values := extractValues(parts[1])
                    if len(values) > 0 {
                        props := make(map[string]interface{})
                        for j, val := range values {
                            props[fmt.Sprintf("col_%d", j)] = val
                        }
                        
                        nodeID := fmt.Sprintf("%s_%d", nodeLabel, i+1)
                        di.store.CreateNode(nodeID, []string{nodeLabel}, props, time.Now().UnixMicro(), 0)
                        stats.NodesCreated++
                    }
                }
            }
        }
        stats.TotalRows++
    }

    stats.Duration = time.Since(startTime).String()
    return stats, nil
}

// ImportFromDatabase imports from live SQL database
func (di *DataImporter) ImportFromDatabase(connString, query, nodeLabel string) (*ImportStats, error) {
    // This would connect to actual databases
    // For MVP, return placeholder
    return &ImportStats{
        TotalRows:    0,
        NodesCreated: 0,
        Errors:       []string{"Database connection not implemented in this example"},
        Duration:     "0s",
    }, nil
}

func parseTimestamp(ts string) (int64, error) {
    // Try various timestamp formats
    formats := []string{
        time.RFC3339,
        "2006-01-02 15:04:05",
        "2006-01-02",
        "2006-01-02T15:04:05Z",
    }
    
    for _, format := range formats {
        if t, err := time.Parse(format, ts); err == nil {
            return t.UnixMicro(), nil
        }
    }
    
    // Try numeric
    if num, err := strconv.ParseInt(ts, 10, 64); err == nil {
        return num, nil
    }
    
    return time.Now().UnixMicro(), nil
}

func extractValues(valuesStr string) []string {
    // Remove parentheses
    valuesStr = strings.Trim(valuesStr, "();")
    valuesStr = strings.TrimSpace(valuesStr)
    
    // Split by comma (simplified - doesn't handle quoted commas)
    parts := strings.Split(valuesStr, ",")
    result := make([]string, len(parts))
    for i, p := range parts {
        result[i] = strings.Trim(p, "' \"")
    }
    return result
}
