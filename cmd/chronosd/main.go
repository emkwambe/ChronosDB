package main

import (
    "flag"
    "log"
    "net"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"

    "google.golang.org/grpc"
    pb "github.com/emkwambe/chronosdb/proto"
    "github.com/emkwambe/chronosdb/internal/api/rest"
    "github.com/emkwambe/chronosdb/internal/storage/temporal"
)

type server struct {
    pb.UnimplementedChronosDBServer
    store *temporal.TemporalStore
}

func (s *server) Execute(req *pb.QueryRequest, stream pb.ChronosDB_ExecuteServer) error {
    log.Printf("gRPC query: %s", req.QueryText)
    
    row := &pb.Row{
        Fields: map[string]*pb.Value{
            "message": {Kind: &pb.Value_StringValue{StringValue: "ChronosDB is running!"}},
            "query":   {Kind: &pb.Value_StringValue{StringValue: req.QueryText}},
        },
    }
    
    return stream.Send(&pb.QueryResponse{
        Result: &pb.QueryResponse_Row{Row: row},
    })
}

func (s *server) Import(stream pb.ChronosDB_ImportServer) error {
    var nodesCreated, edgesCreated int64
    
    for {
        record, err := stream.Recv()
        if err != nil {
            break
        }
        log.Printf("Importing: %s", record.Type)
        if record.Type == "node" {
            nodesCreated++
        } else if record.Type == "edge" {
            edgesCreated++
        }
    }
    
    return stream.SendAndClose(&pb.ImportSummary{
        NodesCreated: nodesCreated,
        EdgesCreated: edgesCreated,
        Errors:       0,
    })
}

func main() {
    // Define command line flags
    grpcPort := flag.String("grpc-port", "50051", "gRPC server port")
    restPort := flag.String("rest-port", "8080", "REST API port")
    dataDir := flag.String("data-dir", "data", "Data directory")
    apiKey := flag.String("api-key", "", "API key for REST authentication (optional)")
    flag.Parse()
    
    // Ensure data directory exists
    dir := *dataDir
    if dir == "" {
        dir = "data"
    }
    
    // Create absolute path
    absPath, err := filepath.Abs(dir)
    if err != nil {
        log.Fatalf("Failed to get absolute path: %v", err)
    }
    
    // Create directory if it doesn't exist
    if err := os.MkdirAll(absPath, 0755); err != nil {
        log.Fatalf("Failed to create data directory: %v", err)
    }
    
    // Initialize temporal store
    log.Printf("Initializing temporal store at %s", absPath)
    store, err := temporal.NewTemporalStore(absPath)
    if err != nil {
        log.Fatalf("Failed to create store: %v", err)
    }
    defer store.Close()
    
    // Start REST API server
    restServer := rest.NewServer(store, *apiKey, *restPort)
    go func() {
        log.Printf("REST API server listening on port %s", *restPort)
        if err := restServer.Start(); err != nil {
            log.Fatalf("REST server failed: %v", err)
        }
    }()
    
    // Start gRPC server
    lis, err := net.Listen("tcp", ":"+*grpcPort)
    if err != nil {
        log.Fatalf("Failed to listen: %v", err)
    }
    
    grpcServer := grpc.NewServer()
    pb.RegisterChronosDBServer(grpcServer, &server{store: store})
    
    go func() {
        log.Printf("gRPC server listening on port %s", *grpcPort)
        if err := grpcServer.Serve(lis); err != nil {
            log.Fatalf("gRPC server failed: %v", err)
        }
    }()
    
    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("Shutting down servers...")
    grpcServer.GracefulStop()
}
