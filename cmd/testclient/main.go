package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "google.golang.org/grpc"
    pb "github.com/emkwambe/chronosdb/proto"
)

func main() {
    conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure(), grpc.WithBlock())
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()

    client := pb.NewChronosDBClient(conn)

    // Test Execute
    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()

    stream, err := client.Execute(ctx, &pb.QueryRequest{
        Database:  "test",
        QueryText: "MATCH (n) RETURN n",
        Parameters: map[string]*pb.Value{},
    })
    if err != nil {
        log.Fatalf("Execute failed: %v", err)
    }

    for {
        resp, err := stream.Recv()
        if err != nil {
            break
        }
        if row := resp.GetRow(); row != nil {
            fmt.Printf("Response: %v\n", row.Fields)
        }
    }

    fmt.Println("Test completed successfully!")
}
