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
	username   string
	guesses    int
	lastGuess  int
	experiment bool
	stream     pb.ExperimentService_ConnectServer // Store the stream to send messages to the client
}

type Server struct {
	pb.UnimplementedExperimentServiceServer
	mu          sync.Mutex
	clients     map[string]*Client // Map of usernames to clients
	targetNum   int
	experiment  bool
	leaderboard map[string]int
}

func NewExperimentServer() *Server {
	return &Server{
		clients:     make(map[string]*Client),
		leaderboard: make(map[string]int),
		experiment:  false,
	}
}

// Connect handles the bi-directional stream when a client connects
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

	client := &Client{username: username, stream: stream, experiment: false}
	s.mu.Lock()
	s.clients[username] = client
	s.mu.Unlock()

	log.Printf("Client '%s' connected", username)

	// Listen for incoming messages (although we're not using further ClientMessage much)
	for {
		_, err := stream.Recv()
		if err != nil {
			log.Printf("Client '%s' disconnected", username)
			s.mu.Lock()
			delete(s.clients, username)
			s.mu.Unlock()
			break
		}
	}

	return nil
}

// StartExperiment broadcasts the start message to all connected clients
func (s *Server) StartExperiment(ctx context.Context, req *pb.StartRequest) (*pb.StartResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Randomly choose a number to guess
	s.targetNum = rand.Intn(100) + 1
	s.experiment = true
	s.leaderboard = make(map[string]int)

	// Notify all connected clients about the start of the experiment
	for _, client := range s.clients {
		client.experiment = true
		go func(c *Client) {
			err := c.stream.Send(&pb.ServerMessage{
				Message: "Experiment started! Guess a number between 1 and 100.",
			})
			if err != nil {
				log.Printf("Failed to notify client '%s'", c.username)
			}
		}(client)
	}

	log.Printf("Experiment started with number: %d", s.targetNum)
	return &pb.StartResponse{Message: "Experiment started!"}, nil
}

// GuessNumber handles incoming guesses from clients and returns a response
func (s *Server) GuessNumber(ctx context.Context, req *pb.GuessRequest) (*pb.GuessResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find the client making the guess based on the username
	client, ok := s.clients[req.Username]
	if !ok {
		return nil, fmt.Errorf("client '%s' not found", req.Username)
	}

	client.guesses++

	var message string
	var hint string
	correct := false

	if req.Number == int32(s.targetNum) {
		message = "Correct!"
		correct = true
		s.leaderboard[req.Username] = client.guesses
	} else if req.Number < int32(s.targetNum) {
		message = "Higher!"
		hint = "Higher"
	} else {
		message = "Lower!"
		hint = "Lower"
	}

	// Respond with the guess result
	return &pb.GuessResponse{
		Message:  message,
		Correct:  correct,
		Attempts: int32(client.guesses),
		Hint:     hint,
	}, nil
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
