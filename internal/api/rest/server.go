package rest

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"

    "github.com/gorilla/mux"
    "github.com/emkwambe/chronosdb/internal/importer"
    "github.com/emkwambe/chronosdb/internal/query/executor"
    "github.com/emkwambe/chronosdb/internal/storage/temporal"
    "github.com/emkwambe/chronosdb/pkg/chronosql"
)

type Server struct {
    router   *mux.Router
    executor *executor.Executor
    apiKey   string
    port     string
    store    *temporal.TemporalStore
}

type QueryRequest struct {
    Query  string                 `json:"query"`
    Params map[string]interface{} `json:"params,omitempty"`
}

type QueryResponse struct {
    Results []executor.Result `json:"results,omitempty"`
    Error   string            `json:"error,omitempty"`
}

func NewServer(store *temporal.TemporalStore, apiKey, port string) *Server {
    exec := executor.NewExecutor(store)
    
    s := &Server{
        router:   mux.NewRouter(),
        executor: exec,
        apiKey:   apiKey,
        port:     port,
        store:    store,
    }
    
    s.routes()
    return s
}

func (s *Server) routes() {
    api := s.router.PathPrefix("/v1/db/{db}").Subrouter()
    api.Use(s.authMiddleware)
    
    api.HandleFunc("/query", s.handleQuery).Methods("POST")
    api.HandleFunc("/health", s.handleHealth).Methods("GET")
    api.HandleFunc("/debug", s.handleDebug).Methods("POST")
    api.HandleFunc("/import", s.handleImportUpload).Methods("POST")
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        apiKey := r.Header.Get("X-API-Key")
        
        if s.apiKey != "" && apiKey != s.apiKey {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusUnauthorized)
            json.NewEncoder(w).Encode(map[string]string{"error": "invalid API key"})
            return
        }
        
        next.ServeHTTP(w, r)
    })
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    
    var req QueryRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(QueryResponse{Error: fmt.Sprintf("invalid request: %v", err)})
        return
    }
    
    if req.Query == "" {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(QueryResponse{Error: "query is required"})
        return
    }
    
    log.Printf("Executing query: %s", req.Query)
    
    results, err := s.executor.Execute(req.Query)
    if err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(QueryResponse{Error: err.Error()})
        return
    }
    
    json.NewEncoder(w).Encode(QueryResponse{Results: results})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (s *Server) handleDebug(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    
    var req QueryRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return
    }
    
    parser := chronosql.NewParser()
    query, err := parser.Parse(req.Query)
    if err != nil {
        json.NewEncoder(w).Encode(map[string]interface{}{
            "error": err.Error(),
            "query": req.Query,
        })
        return
    }
    
    json.NewEncoder(w).Encode(map[string]interface{}{
        "type":     query.Type,
        "pattern":  query.Pattern,
        "temporal": query.Temporal,
        "where":    query.Where,
        "original": req.Query,
    })
}

func (s *Server) handleImportUpload(w http.ResponseWriter, r *http.Request) {
    if err := r.ParseMultipartForm(32 << 20); err != nil {
        respondError(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
        return
    }
    
    file, _, err := r.FormFile("file")
    if err != nil {
        respondError(w, "No file uploaded", http.StatusBadRequest)
        return
    }
    defer file.Close()
    
    label := r.FormValue("label")
    format := r.FormValue("format")
    
    if label == "" {
        respondError(w, "Label is required", http.StatusBadRequest)
        return
    }
    
    content, err := io.ReadAll(file)
    if err != nil {
        respondError(w, "Failed to read file: "+err.Error(), http.StatusInternalServerError)
        return
    }
    
    dataImporter := importer.NewDataImporter(s.store, s.executor)
    var stats *importer.ImportStats
    
    switch format {
    case "csv":
        tempFile, _ := os.CreateTemp("", "upload-*.csv")
        defer os.Remove(tempFile.Name())
        tempFile.Write(content)
        tempFile.Close()
        stats, err = dataImporter.ImportCSV(tempFile.Name(), label, "")
    case "json":
        tempFile, _ := os.CreateTemp("", "upload-*.json")
        defer os.Remove(tempFile.Name())
        tempFile.Write(content)
        tempFile.Close()
        stats, err = dataImporter.ImportJSON(tempFile.Name(), label)
    default:
        respondError(w, "Unsupported format: "+format, http.StatusBadRequest)
        return
    }
    
    if err != nil {
        respondError(w, "Import failed: "+err.Error(), http.StatusInternalServerError)
        return
    }
    
    respondJSON(w, stats, http.StatusOK)
}

func (s *Server) Start() error {
    return http.ListenAndServe(":"+s.port, s.router)
}

func respondJSON(w http.ResponseWriter, data interface{}, status int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, message string, status int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{"error": message})
}
