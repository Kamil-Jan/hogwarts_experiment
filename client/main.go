package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	pb "github.com/Kamil-Jan/hogwarts_experiment/proto"
	"google.golang.org/grpc"
)

// Client struct to encapsulate connection and stream
type Client struct {
	conn     *grpc.ClientConn
	client   pb.ExperimentServiceClient
	stream   pb.ExperimentService_ConnectClient
	ctx      context.Context
	cancel   context.CancelFunc
	username string
}

// NewClient initializes the client with a connection to the server and registers the username
func NewClient(serverAddr, username string) (*Client, error) {
	// Setup a gRPC connection
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	// Initialize the gRPC client
	client := pb.NewExperimentServiceClient(conn)

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Establish a stream with the server
	stream, err := client.Connect(ctx)
	if err != nil {
		conn.Close() // Ensure connection is closed if stream creation fails
		cancel()     // Cancel context if the connection fails
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	// Send the username to the server
	err = stream.Send(&pb.ConnectRequest{Username: username})
	if err != nil {
		conn.Close()
		cancel()
		return nil, fmt.Errorf("failed to send username: %w", err)
	}

	return &Client{
		conn:     conn,
		client:   client,
		stream:   stream,
		ctx:      ctx,
		cancel:   cancel,
		username: username,
	}, nil
}

// Close gracefully closes the client connection and cancels the context
func (c *Client) Close() {
	c.cancel()
	c.conn.Close()
}

// ListenForMessages continuously listens for messages from the server
func (c *Client) ListenForMessages(startSignal chan struct{}) {
	for {
		serverMsg, err := c.stream.Recv()
		if err != nil {
			log.Printf("Failed to receive message from server: %v", err)
			break
		}
		fmt.Printf("Server: %s\n", serverMsg.Message)

		// Notify client when the experiment starts
		if strings.Contains(serverMsg.Message, "Experiment started") {
			close(startSignal)
		}
	}
}

// GuessNumber sends the guessed number to the server and gets the result
func (c *Client) GuessNumber(guess int32) (*pb.GuessResponse, error) {
	// Create a context with timeout to avoid hanging indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make the GuessNumber RPC call
	response, err := c.client.GuessNumber(ctx, &pb.GuessRequest{Username: c.username, Number: guess})
	if err != nil {
		return nil, fmt.Errorf("error guessing number: %w", err)
	}

	return response, nil
}

// HandleUserInput handles the guessing logic and interaction with the server
func (c *Client) HandleUserInput(startSignal chan struct{}) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Waiting for the experiment to start...")

	// Wait for the experiment to start
	<-startSignal
	fmt.Println("Experiment started! Enter your guesses.")

	for {
		fmt.Print("Enter your guess (1-100): ")
		guessStr, _ := reader.ReadString('\n')
		guessStr = strings.TrimSpace(guessStr)

		// Validate the input
		guess, err := strconv.Atoi(guessStr)
		if err != nil || guess < 1 || guess > 100 {
			fmt.Println("Invalid input. Please enter a number between 1 and 100.")
			continue
		}

		// Send the guess to the server using GuessNumber RPC
		response, err := c.GuessNumber(int32(guess))
		if err != nil {
			log.Printf("Failed to send guess: %v", err)
			return
		}

		// Handle the server's response
		if response.Correct {
			fmt.Printf("Correct! You guessed the number in %d attempts.\n", response.Attempts)
			return
		} else {
			fmt.Printf("%s (Hint: %s)\n", response.Message, response.Hint)
		}
	}
}

func main() {
	// Get the username from the user
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	// Address of the server
	serverAddr := "localhost:50051"

	// Initialize client with the username
	client, err := NewClient(serverAddr, username)
	if err != nil {
		log.Fatalf("Error initializing client: %v", err)
	}
	defer client.Close()

	// Channel to signal the start of the experiment
	startSignal := make(chan struct{})

	// Listen for messages from the server in a separate goroutine
	go client.ListenForMessages(startSignal)

	// Gracefully handle system interrupts for cleanup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\nReceived interrupt signal, shutting down gracefully...")
		client.Close()
		os.Exit(0)
	}()

	// Handle user input for guessing
	client.HandleUserInput(startSignal)
}
