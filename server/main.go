package main

import (
	"context"
	"fmt"
	"io"
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
	if _, ok := s.leaderboard[username]; !ok {
		s.leaderboard[username] = 0
	}
	s.mu.Unlock()

	log.Printf("Client '%s' connected", username)

	// Listen for guesses from the client
	for {
		clientMsg, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				log.Printf("Error receiving message from client '%s': %v", username, err)
			}
			break
		}

		// Process the client's guess but do not send an immediate response
		s.processGuess(username, clientMsg.Number)
	}

	log.Printf("Client '%s' disconnected", username)

	// Remove the client after disconnect
	s.mu.Lock()
	delete(s.pendingResponses, username)
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

	if s.experiment {
		return nil, fmt.Errorf("experiment has already started")
	}

	// Generate a random number for the experiment
	s.targetNum = rand.Intn(100) + 1
	s.experiment = true
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

// EndExperiment ends the current experiment, notifies all clients, and optionally returns the final leaderboard
func (s *Server) EndExperiment(ctx context.Context, req *pb.EndRequest) (*pb.EndResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if there is an active experiment
	if !s.experiment {
		return nil, fmt.Errorf("no active experiment to end")
	}

	// Notify all clients that the experiment is over
	for _, client := range s.clients {
		err := client.stream.Send(&pb.ServerMessage{
			Message: "Experiment ended!",
		})
		if err != nil {
			log.Printf("Failed to notify client '%s' about experiment end: %v", client.username, err)
		}
	}

	// Clear experiment state
	s.experiment = false
	s.targetNum = 0
	s.pendingResponses = make(map[string]int32) // Clear pending responses
	log.Println("Experiment ended.")

	// Optionally, return the final leaderboard to the admin
	leaderboardMsg := "Final leaderboard:\n"
	for username, attempts := range s.leaderboard {
		leaderboardMsg += fmt.Sprintf("%s: %d attempts\n", username, attempts)
	}

	return &pb.EndResponse{Message: leaderboardMsg}, nil
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
		s.leaderboard[req.Username] += 1
	} else if guess < int32(s.targetNum) {
		message = "Higher!"
	} else {
		message = "Lower!"
	}
	delete(s.pendingResponses, req.Username)

	// Send the response to the client
	err := client.stream.Send(&pb.ServerMessage{Message: message})
	if err != nil {
		return nil, fmt.Errorf("failed to send message to client '%s': %v", req.Username, err)
	}

	log.Printf("Sent response to client '%s': %s", req.Username, message)

	return &pb.SendResponseResponse{Message: "Response sent to client"}, nil
}

// Leaderboard returns the current leaderboard
func (s *Server) Leaderboard(ctx context.Context, req *pb.LeaderboardRequest) (*pb.LeaderboardResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries := []*pb.LeaderboardEntry{}
	for username, wins := range s.leaderboard {
		entries = append(entries, &pb.LeaderboardEntry{
			Username: username,
			Wins:     int32(wins),
		})
	}

	return &pb.LeaderboardResponse{Entries: entries}, nil
}

// WaitingList returns the list of clients who are waiting for a response
func (s *Server) WaitingList(ctx context.Context, req *pb.WaitingListRequest) (*pb.WaitingListResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	usernames := []string{}
	for username := range s.pendingResponses {
		usernames = append(usernames, username)
	}

	return &pb.WaitingListResponse{Usernames: usernames}, nil
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
