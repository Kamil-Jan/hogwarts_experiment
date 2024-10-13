package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"

	pb "github.com/Kamil-Jan/hogwarts_experiment/proto"
)

// Client holds information about connected clients
type Client struct {
	id         string
	guesses    int
	lastGuess  int
	stream     pb.ExperimentService_ConnectServer
	experiment bool
}

// Server holds the server's state
type Server struct {
	pb.UnimplementedExperimentServiceServer
	mu          sync.Mutex
	clients     map[string]*Client
	targetNum   int
	experiment  bool
	leaderboard map[string]int
}

// NewExperimentServer initializes the server instance
func NewExperimentServer() *Server {
	return &Server{
		clients:     make(map[string]*Client),
		leaderboard: make(map[string]int),
		experiment:  false,
	}
}

// Connect allows clients to register and interact with the server
func (s *Server) Connect(stream pb.ExperimentService_ConnectServer) error {
	// Generate a unique ID for the client
	clientID := fmt.Sprintf("Client-%d", rand.Intn(1000))

	// Register the client
	client := &Client{id: clientID, stream: stream, experiment: false}
	s.mu.Lock()
	s.clients[clientID] = client
	s.mu.Unlock()

	log.Printf("Client %s connected", clientID)

	// Listen for guesses from the client
	for {
		// Receive a guess from the client
		guessMsg, err := stream.Recv()
		if err != nil {
			log.Printf("Client %s disconnected", clientID)
			s.mu.Lock()
			delete(s.clients, clientID)
			s.mu.Unlock()
			break
		}

		// Process the guess
		s.handleGuess(clientID, guessMsg.Number)
	}

	return nil
}

// StartExperiment starts a new experiment with a random number
func (s *Server) StartExperiment(ctx context.Context, req *pb.StartRequest) (*pb.StartResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate a new random number between 1 and 100
	s.targetNum = rand.Intn(100) + 1
	s.experiment = true
	s.leaderboard = make(map[string]int)

	// Notify all connected clients about the experiment
	for _, client := range s.clients {
		client.experiment = true
		go func(c *Client) {
			err := c.stream.Send(&pb.ServerMessage{
				Message: "Experiment started! Guess a number between 1 and 100.",
			})
			if err != nil {
				log.Printf("Failed to notify %s", c.id)
			}
		}(client)
	}

	log.Printf("Experiment started with number: %d", s.targetNum)

	return &pb.StartResponse{Message: "Experiment started!"}, nil
}

// handleGuess processes a guess from a client
func (s *Server) handleGuess(clientID string, guess int32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client := s.clients[clientID]
	client.guesses++
	client.lastGuess = int(guess)

	response := &pb.ServerMessage{}

	// Compare the guess with the target number
	if guess == int32(s.targetNum) {
		response.Message = "You guessed it!"
		s.leaderboard[clientID] = client.guesses
		client.experiment = false
	} else if guess < int32(s.targetNum) {
		response.Message = "Higher!"
	} else {
		response.Message = "Lower!"
	}

	// Send feedback to the client
	err := client.stream.Send(response)
	if err != nil {
		log.Printf("Failed to send response to %s", client.id)
	}
}

// Leaderboard returns the current leaderboard
func (s *Server) Leaderboard(ctx context.Context, req *pb.LeaderboardRequest) (*pb.LeaderboardResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Construct the leaderboard
	leaderboard := []*pb.LeaderboardEntry{}
	for id, attempts := range s.leaderboard {
		leaderboard = append(leaderboard, &pb.LeaderboardEntry{
			ClientId: id,
			Attempts: int32(attempts),
		})
	}

	return &pb.LeaderboardResponse{Entries: leaderboard}, nil
}
