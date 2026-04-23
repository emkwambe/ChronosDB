package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/emkwambe/chronosdb/internal/api/rest"
	"github.com/emkwambe/chronosdb/internal/query/executor"
	"github.com/emkwambe/chronosdb/internal/storage/temporal"
	pb "github.com/emkwambe/chronosdb/proto"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedChronosDBServer
	store    *temporal.TemporalStore
	executor *executor.Executor
}

func (s *server) Execute(req *pb.QueryRequest, stream pb.ChronosDB_ExecuteServer) error {
	log.Printf("gRPC query: %s", req.QueryText)

	results, err := s.executor.Execute(req.QueryText)
	if err != nil {
		return err
	}

	for _, result := range results {
		row := &pb.Row{
			Fields: map[string]*pb.Value{
				"type": {Kind: &pb.Value_StringValue{StringValue: result.Type}},
			},
		}
		if err := stream.Send(&pb.QueryResponse{Result: &pb.QueryResponse_Row{Row: row}}); err != nil {
			return err
		}
	}

	return nil
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
	grpcPort := flag.String("grpc-port", "50051", "gRPC server port")
	restPort := flag.String("rest-port", "8080", "REST API port")
	dataDir := flag.String("data-dir", "data", "Data directory")
	apiKey := flag.String("api-key", "", "API key for REST authentication")

	flag.Parse()

	// Ensure data directory exists
	absPath, err := filepath.Abs(*dataDir)
	if err != nil {
		log.Fatalf("Failed to get absolute path: %v", err)
	}
	if err := os.MkdirAll(absPath, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Initialize store
	log.Printf("Initializing temporal store at %s", absPath)
	store, err := temporal.NewTemporalStore(absPath)
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create executor
	exec := executor.NewExecutor(store)

	// Start REST API
	restServer := rest.NewServer(store, *apiKey, *restPort)
	go func() {
		log.Printf("REST API listening on port %s", *restPort)
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
	pb.RegisterChronosDBServer(grpcServer, &server{store: store, executor: exec})

	go func() {
		log.Printf("gRPC server listening on port %s", *grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// Wait for shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down servers...")
	grpcServer.GracefulStop()
}
