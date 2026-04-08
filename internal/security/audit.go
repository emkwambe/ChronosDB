package security

import (
    "encoding/json"
    "log"
    "os"
    "sync"
    "time"
)

type AuditEntry struct {
    Timestamp   int64                  `json:"timestamp"`
    UserID      string                 `json:"user_id"`
    Action      string                 `json:"action"`      // "query", "create", "delete", "import"
    Query       string                 `json:"query,omitempty"`
    Database    string                 `json:"database"`
    Status      string                 `json:"status"`      // "success", "error"
    Error       string                 `json:"error,omitempty"`
    DurationMs  int64                  `json:"duration_ms"`
    Details     map[string]interface{} `json:"details,omitempty"`
}

type AuditLogger struct {
    file   *os.File
    mutex  sync.Mutex
    logger *log.Logger
}

var globalAuditLogger *AuditLogger

func InitAuditLogger(logPath string) error {
    file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return err
    }
    
    globalAuditLogger = &AuditLogger{
        file:   file,
        logger: log.New(file, "", 0),
    }
    
    return nil
}

func GetAuditLogger() *AuditLogger {
    return globalAuditLogger
}

func (a *AuditLogger) Log(entry AuditEntry) {
    a.mutex.Lock()
    defer a.mutex.Unlock()
    
    if entry.Timestamp == 0 {
        entry.Timestamp = time.Now().UnixMicro()
    }
    
    data, err := json.Marshal(entry)
    if err != nil {
        a.logger.Printf("ERROR: Failed to marshal audit entry: %v", err)
        return
    }
    
    a.logger.Println(string(data))
}

func (a *AuditLogger) Close() error {
    if a.file != nil {
        return a.file.Close()
    }
    return nil
}

func LogQuery(userID, database, query string, durationMs int64, err error) {
    if globalAuditLogger == nil {
        return
    }
    
    status := "success"
    errMsg := ""
    if err != nil {
        status = "error"
        errMsg = err.Error()
    }
    
    globalAuditLogger.Log(AuditEntry{
        UserID:     userID,
        Action:     "query",
        Query:      query,
        Database:   database,
        Status:     status,
        Error:      errMsg,
        DurationMs: durationMs,
    })
}
