package main

import (
	"log"
	"net"

	pb "github.com/Kamil-Jan/hogwarts_experiment/proto"

	"google.golang.org/grpc"
)

func main() {
	// Create a new gRPC server
	grpcServer := grpc.NewServer()

	// Initialize and register the experiment service server
	server := NewExperimentServer() // Calls the function from server.go
	pb.RegisterExperimentServiceServer(grpcServer, server)

	// Start listening on port 50051
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen on port 50051: %v", err)
	}

	log.Println("Server is listening on port 50051...")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}
