package main

import (
    "bytes"
    "encoding/csv"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "net/http"
    "os"
    "strconv"
    "strings"
    "time"
)

type ImportStats struct {
    TotalRows    int      `json:"total_rows"`
    NodesCreated int      `json:"nodes_created"`
    EdgesCreated int      `json:"edges_created"`
    Errors       []string `json:"errors,omitempty"`
    Duration     string   `json:"duration"`
}

func main() {
    filePath := flag.String("file", "", "Path to data file")
    format := flag.String("format", "csv", "csv, json, or sql")
    nodeLabel := flag.String("label", "Data", "Node label")
    apiURL := flag.String("api", "http://localhost:8080", "ChronosDB API URL")
    timestampCol := flag.String("timestamp", "", "Timestamp column name")
    flag.Parse()

    if *filePath == "" {
        fmt.Println("Error: -file is required")
        return
    }

    // Read file
    data, err := os.ReadFile(*filePath)
    if err != nil {
        fmt.Printf("Error reading file: %v\n", err)
        return
    }

    startTime := time.Now()
    var stats *ImportStats
    var importErr error

    switch *format {
    case "csv":
        stats, importErr = importCSV(data, *nodeLabel, *timestampCol, *apiURL)
    case "json":
        stats, importErr = importJSON(data, *nodeLabel, *apiURL)
    case "sql":
        stats, importErr = importSQL(data, *nodeLabel, *apiURL)
    default:
        fmt.Printf("Unsupported format: %s\n", *format)
        return
    }

    if importErr != nil {
        fmt.Printf("Import failed: %v\n", importErr)
        return
    }

    stats.Duration = time.Since(startTime).String()

    // Print results
    fmt.Printf("\n=== Import Results ===\n")
    fmt.Printf("Total Rows: %d\n", stats.TotalRows)
    fmt.Printf("Nodes Created: %d\n", stats.NodesCreated)
    fmt.Printf("Duration: %s\n", stats.Duration)
    if len(stats.Errors) > 0 {
        fmt.Printf("Errors: %d\n", len(stats.Errors))
    }
}

func importCSV(data []byte, label, timestampCol, apiURL string) (*ImportStats, error) {
    reader := csv.NewReader(bytes.NewReader(data))
    headers, err := reader.Read()
    if err != nil {
        return nil, fmt.Errorf("failed to read headers: %w", err)
    }

    stats := &ImportStats{}
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

        // Build properties
        props := make(map[string]interface{})
        for i, header := range headers {
            if i < len(record) && record[i] != "" {
                if header == timestampCol {
                    continue
                }
                if num, err := strconv.ParseFloat(record[i], 64); err == nil {
                    props[header] = num
                } else {
                    props[header] = record[i]
                }
            }
        }

        propsJSON, _ := json.Marshal(props)
        query := fmt.Sprintf("CREATE (n:%s %s)", label, string(propsJSON))

        if err := executeQuery(query, apiURL); err != nil {
            stats.Errors = append(stats.Errors, fmt.Sprintf("Row %d: %v", rowNum, err))
        } else {
            stats.NodesCreated++
        }
        stats.TotalRows++
        rowNum++
    }

    return stats, nil
}

func importJSON(data []byte, label, apiURL string) (*ImportStats, error) {
    var records []map[string]interface{}
    if err := json.Unmarshal(data, &records); err != nil {
        return nil, fmt.Errorf("failed to parse JSON: %w", err)
    }

    stats := &ImportStats{}

    for i, record := range records {
        delete(record, "timestamp")
        propsJSON, _ := json.Marshal(record)
        query := fmt.Sprintf("CREATE (n:%s %s)", label, string(propsJSON))

        if err := executeQuery(query, apiURL); err != nil {
            stats.Errors = append(stats.Errors, fmt.Sprintf("Record %d: %v", i+1, err))
        } else {
            stats.NodesCreated++
        }
        stats.TotalRows++
    }

    return stats, nil
}

func importSQL(data []byte, label, apiURL string) (*ImportStats, error) {
    lines := strings.Split(string(data), "\n")
    stats := &ImportStats{}

    for i, line := range lines {
        if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "INSERT") {
            props := make(map[string]interface{})
            props["sql_row"] = i
            props["raw_data"] = line

            propsJSON, _ := json.Marshal(props)
            query := fmt.Sprintf("CREATE (n:%s %s)", label, string(propsJSON))

            if err := executeQuery(query, apiURL); err != nil {
                stats.Errors = append(stats.Errors, fmt.Sprintf("Line %d: %v", i+1, err))
            } else {
                stats.NodesCreated++
            }
        }
        stats.TotalRows++
    }

    return stats, nil
}

func executeQuery(query, apiURL string) error {
    payload := map[string]string{"query": query}
    body, _ := json.Marshal(payload)

    resp, err := http.Post(apiURL+"/v1/db/test/query", "application/json", bytes.NewBuffer(body))
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("API returned status %d", resp.StatusCode)
    }

    return nil
}
