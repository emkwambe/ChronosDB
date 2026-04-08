package chronosql

import (
    "fmt"
    "strconv"
    "strings"
    "time"
)

type QueryType string

const (
    TypeMatch   QueryType = "MATCH"
    TypeCreate  QueryType = "CREATE"
    TypeUpdate  QueryType = "UPDATE"
    TypeDelete  QueryType = "DELETE"
)

type TemporalClause struct {
    Type      string
    Timestamp int64
    StartTime int64
    EndTime   int64
}

type Query struct {
    Type     QueryType
    Pattern  string
    Returns  []string
    Temporal TemporalClause
    Where    map[string]interface{}
}

type Parser struct{}

func NewParser() *Parser {
    return &Parser{}
}

func (p *Parser) Parse(queryStr string) (*Query, error) {
    queryStr = strings.TrimSpace(queryStr)
    query := &Query{
        Where: make(map[string]interface{}),
    }
    
    // Handle DELETE queries specially
    upperQuery := strings.ToUpper(queryStr)
    if strings.HasPrefix(upperQuery, "DELETE") {
        query.Type = TypeDelete
        remaining := strings.TrimSpace(queryStr[6:])
        
        // Parse temporal clause if present
        temporal, remaining, err := p.parseTemporalClause(remaining)
        if err != nil {
            return nil, err
        }
        query.Temporal = temporal
        
        // Extract pattern
        remaining = strings.TrimSpace(remaining)
        if idx := strings.Index(strings.ToUpper(remaining), "WHERE"); idx != -1 {
            query.Pattern = strings.TrimSpace(remaining[:idx])
            whereClause := strings.TrimSpace(remaining[idx+5:])
            // Parse WHERE clause (simplified)
            query.Where["condition"] = whereClause
        } else {
            query.Pattern = remaining
        }
        
        return query, nil
    }
    
    // Handle other query types
    parts := strings.SplitN(queryStr, " ", 2)
    if len(parts) < 2 {
        return nil, fmt.Errorf("invalid query: %s", queryStr)
    }
    
    query.Type = QueryType(strings.ToUpper(parts[0]))
    remaining := parts[1]
    
    // Parse temporal clause
    temporal, remaining, err := p.parseTemporalClause(remaining)
    if err != nil {
        return nil, err
    }
    query.Temporal = temporal
    
    // Extract pattern (everything before RETURN)
    remaining = strings.TrimSpace(remaining)
    if idx := strings.Index(strings.ToUpper(remaining), "RETURN"); idx != -1 {
        patternPart := strings.TrimSpace(remaining[:idx])
        query.Pattern = patternPart
        returnPart := strings.TrimSpace(remaining[idx+6:])
        query.Returns = strings.Split(returnPart, ",")
        for i, r := range query.Returns {
            query.Returns[i] = strings.TrimSpace(r)
        }
    } else {
        query.Pattern = remaining
        query.Returns = []string{}
    }
    
    // Remove WHERE clause from pattern if present
    if idx := strings.Index(strings.ToUpper(query.Pattern), "WHERE"); idx != -1 {
        query.Pattern = strings.TrimSpace(query.Pattern[:idx])
    }
    
    return query, nil
}

func (p *Parser) parseTemporalClause(query string) (TemporalClause, string, error) {
    temporal := TemporalClause{}
    upperQuery := strings.ToUpper(query)
    
    if idx := strings.Index(upperQuery, "AS OF"); idx != -1 {
        temporal.Type = "AS_OF"
        afterAsOf := strings.TrimSpace(query[idx+5:])
        
        if strings.HasPrefix(afterAsOf, "'") || strings.HasPrefix(afterAsOf, "\"") {
            quote := afterAsOf[0]
            endIdx := strings.IndexByte(afterAsOf[1:], quote)
            if endIdx == -1 {
                return temporal, "", fmt.Errorf("unclosed quote in AS OF")
            }
            timeStr := afterAsOf[1 : endIdx+1]
            t, err := p.parseTimestamp(timeStr)
            if err != nil {
                return temporal, "", err
            }
            temporal.Timestamp = t
            remaining := strings.TrimSpace(afterAsOf[endIdx+2:])
            return temporal, remaining, nil
        }
        
        parts := strings.Fields(afterAsOf)
        if len(parts) == 0 {
            return temporal, "", fmt.Errorf("missing timestamp in AS OF")
        }
        t, err := strconv.ParseInt(parts[0], 10, 64)
        if err != nil {
            return temporal, "", fmt.Errorf("invalid timestamp: %s", parts[0])
        }
        temporal.Timestamp = t
        remaining := strings.TrimSpace(strings.Join(parts[1:], " "))
        return temporal, remaining, nil
    }
    
    if idx := strings.Index(upperQuery, "BETWEEN"); idx != -1 {
        temporal.Type = "BETWEEN"
        afterBetween := strings.TrimSpace(query[idx+7:])
        
        parts := strings.Fields(afterBetween)
        if len(parts) < 3 {
            return temporal, "", fmt.Errorf("invalid BETWEEN clause")
        }
        
        start, err := p.parseTimestamp(parts[0])
        if err != nil {
            return temporal, "", err
        }
        
        if strings.ToUpper(parts[1]) != "AND" {
            return temporal, "", fmt.Errorf("expected AND in BETWEEN clause")
        }
        
        end, err := p.parseTimestamp(parts[2])
        if err != nil {
            return temporal, "", err
        }
        
        temporal.StartTime = start
        temporal.EndTime = end
        
        remaining := strings.TrimSpace(strings.Join(parts[3:], " "))
        return temporal, remaining, nil
    }
    
    return temporal, query, nil
}

func (p *Parser) parseTimestamp(ts string) (int64, error) {
    if num, err := strconv.ParseInt(ts, 10, 64); err == nil {
        return num, nil
    }
    
    formats := []string{
        time.RFC3339,
        "2006-01-02",
        "2006-01-02T15:04:05",
        "2006-01-02 15:04:05",
    }
    
    for _, format := range formats {
        if t, err := time.Parse(format, ts); err == nil {
            return t.UnixMicro(), nil
        }
    }
    
    return 0, fmt.Errorf("unsupported timestamp format: %s", ts)
}

func FormatTimestamp(micros int64) string {
    if micros == 0 {
        return "beginning of time"
    }
    t := time.UnixMicro(micros)
    return t.Format(time.RFC3339)
}
