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
	id         string
	guesses    int
	lastGuess  int
	experiment bool
}

type Server struct {
	pb.UnimplementedExperimentServiceServer
	mu          sync.Mutex
	clients     map[string]*Client
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

func (s *Server) Connect(stream pb.ExperimentService_ConnectServer) error {
	clientID := fmt.Sprintf("Client-%d", rand.Intn(1000))
	log.Printf("Client %s connected", clientID)

	// Register new client
	s.mu.Lock()
	s.clients[clientID] = &Client{id: clientID}
	s.mu.Unlock()

	// Listen for incoming messages from the client (although we're not using ClientMessage much)
	for {
		_, err := stream.Recv()
		if err != nil {
			log.Printf("Client %s disconnected", clientID)
			s.mu.Lock()
			delete(s.clients, clientID)
			s.mu.Unlock()
			break
		}
	}

	return nil
}

func (s *Server) StartExperiment(ctx context.Context, req *pb.StartRequest) (*pb.StartResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.targetNum = rand.Intn(100) + 1
	s.experiment = true
	s.leaderboard = make(map[string]int)

	for _, client := range s.clients {
		client.experiment = true
		go func(c *Client) {
			err := s.notifyClient(c, "Experiment started! Guess a number between 1 and 100.")
			if err != nil {
				log.Printf("Failed to notify client %s", c.id)
			}
		}(client)
	}

	log.Printf("Experiment started with number: %d", s.targetNum)
	return &pb.StartResponse{Message: "Experiment started!"}, nil
}

func (s *Server) notifyClient(client *Client, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stream, err := client.stream.Recv()
	if err != nil {
		return fmt.Errorf("error receiving stream: %v", err)
	}

	err = stream.Send(&pb.ServerMessage{Message: message})
	if err != nil {
		return fmt.Errorf("error sending message: %v", err)
	}

	return nil
}

func (s *Server) GuessNumber(ctx context.Context, req *pb.GuessRequest) (*pb.GuessResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Handle the guess
	clientID := fmt.Sprintf("Client-%d", rand.Intn(1000)) // for now
	client := s.clients[clientID]

	client.guesses++

	var message string
	var hint string
	correct := false

	if req.Number == int32(s.targetNum) {
		message = "Correct!"
		correct = true
		s.leaderboard[clientID] = client.guesses
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
