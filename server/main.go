package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"

	pb "github.com/Kamil-Jan/hogwarts_experiment/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Client struct {
	username  string
	guesses   int
	lastGuess int32
	stream    pb.ExperimentService_ConnectServer // Store the stream to send messages to the client
}

type Server struct {
	pb.UnimplementedExperimentServiceServer
	mu               sync.Mutex
	clients          map[string]*Client // Map of usernames to clients
	targetNum        int
	experiment       bool
	leaderboard      map[string]int
	pendingResponses map[string]int32 // Store guesses awaiting responses for each client
}

func NewExperimentServer() *Server {
	return &Server{
		clients:          make(map[string]*Client),
		leaderboard:      make(map[string]int),
		pendingResponses: make(map[string]int32), // Track pending guesses for each client
	}
}

// Connect handles bidirectional streaming between the server and client
func (s *Server) Connect(stream pb.ExperimentService_ConnectServer) error {
	// Receive the first message from the client containing the username
	clientMsg, err := stream.Recv()
	if err != nil {
		log.Printf("Error receiving username: %v", err)
		return err
	}

	// Register a new client with the provided username and stream
	username := clientMsg.Username
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	client := &Client{username: username, stream: stream}
	s.mu.Lock()
	s.clients[username] = client
	s.mu.Unlock()

	log.Printf("Client '%s' connected", username)

	// Listen for guesses from the client
	for {
		clientMsg, err := stream.Recv()
		if err != nil {
			log.Printf("Error receiving message from client '%s': %v", username, err)
			break
		}

		// Process the client's guess but do not send an immediate response
		s.processGuess(username, clientMsg.Number)
	}

	// Remove the client after disconnect
	s.mu.Lock()
	delete(s.clients, username)
	s.mu.Unlock()

	return nil
}

// processGuess stores the guess for later response
func (s *Server) processGuess(username string, guess int32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, ok := s.clients[username]
	if !ok {
		log.Printf("Client '%s' not found", username)
		return
	}

	client.guesses++
	client.lastGuess = guess

	// Store the guess in the pending responses map for manual response later
	s.pendingResponses[username] = guess
	log.Printf("Stored guess %d for client '%s' (pending response)", guess, username)
}

// StartExperiment sends a start message to all clients
func (s *Server) StartExperiment(ctx context.Context, req *pb.StartRequest) (*pb.StartResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate a random number for the experiment
	s.targetNum = rand.Intn(100) + 1
	s.experiment = true
	s.leaderboard = make(map[string]int)
	log.Printf("Experiment started with number: %d", s.targetNum)

	// Notify all clients about the start of the experiment
	for _, client := range s.clients {
		err := client.stream.Send(&pb.ServerMessage{
			Message: "Experiment started! Guess a number between 1 and 100.",
		})
		if err != nil {
			log.Printf("Error sending start message to client '%s': %v", client.username, err)
		}
	}

	return &pb.StartResponse{Message: "Experiment started!"}, nil
}

// SendResponse sends a response for the last guess of a specific client
func (s *Server) SendResponse(ctx context.Context, req *pb.SendResponseRequest) (*pb.SendResponseResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, ok := s.clients[req.Username]
	if !ok {
		return nil, fmt.Errorf("client '%s' not found", req.Username)
	}

	// Get the stored guess for the client
	guess, exists := s.pendingResponses[req.Username]
	if !exists {
		return nil, fmt.Errorf("no pending response for client '%s'", req.Username)
	}

	// Process the guess (manual response based on guess)
	var message string
	if guess == int32(s.targetNum) {
		message = "Correct!"
		s.leaderboard[req.Username] = client.guesses
		delete(s.pendingResponses, req.Username)
	} else if guess < int32(s.targetNum) {
		message = "Higher!"
	} else {
		message = "Lower!"
	}

	// Send the response to the client
	err := client.stream.Send(&pb.ServerMessage{Message: message})
	if err != nil {
		return nil, fmt.Errorf("failed to send message to client '%s': %v", req.Username, err)
	}

	log.Printf("Sent response to client '%s': %s", req.Username, message)

	return &pb.SendResponseResponse{Message: "Response sent to client"}, nil
}

func main() {
	grpcServer := grpc.NewServer()

	server := NewExperimentServer()
	pb.RegisterExperimentServiceServer(grpcServer, server)

	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen on port 50051: %v", err)
	}

	log.Println("Server is listening on port 50051...")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}
