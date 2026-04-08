package rest

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"

    "github.com/gorilla/mux"
    "github.com/emkwambe/chronosdb/internal/query/executor"
    "github.com/emkwambe/chronosdb/internal/storage/temporal"
    "github.com/emkwambe/chronosdb/pkg/chronosql"
)

type Server struct {
    router   *mux.Router
    executor *executor.Executor
    apiKey   string
    port     string
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
    }
    
    s.routes()
    return s
}

func (s *Server) routes() {
    api := s.router.PathPrefix("/v1/db/{db}").Subrouter()
    api.Use(s.authMiddleware)
    
    api.HandleFunc("/query", s.handleQuery).Methods("POST")`n    api.HandleFunc("/import", s.handleImport).Methods("POST")
    api.HandleFunc("/health", s.handleHealth).Methods("GET")
    api.HandleFunc("/debug", s.handleDebug).Methods("POST")
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

func (s *Server) Start() error {
    return http.ListenAndServe(":"+s.port, s.router)
}

