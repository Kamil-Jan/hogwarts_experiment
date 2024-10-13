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

	pb "github.com/Kamil-Jan/hogwarts_experiment/proto"
	"google.golang.org/grpc"
)

type Client struct {
	conn     *grpc.ClientConn
	client   pb.ExperimentServiceClient
	stream   pb.ExperimentService_ConnectClient
	username string
	start    chan struct{}
	msg      chan string
}

// NewClient initializes the client and establishes a connection with the server
func NewClient(serverAddr, username string) (*Client, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	client := pb.NewExperimentServiceClient(conn)
	stream, err := client.Connect(context.Background())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	// Send the initial message with the username
	err = stream.Send(&pb.ClientMessage{Username: username})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send username: %w", err)
	}

	return &Client{
		conn:     conn,
		client:   client,
		stream:   stream,
		username: username,
		start:    make(chan struct{}),
		msg:      make(chan string),
	}, nil
}

// Close gracefully closes the client connection
func (c *Client) Close() {
	c.stream.CloseSend()
	c.conn.Close()
}

func (c *Client) Guess() {
	reader := bufio.NewReader(os.Stdin)
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

		// Send the guess to the server using the stream
		err = c.stream.Send(&pb.ClientMessage{
			Username: c.username,
			Number:   int32(guess),
		})
		if err != nil {
			log.Printf("Failed to send guess: %v", err)
		}
		break
	}
}

// ListenAndInteract listens for responses from the server and interacts with the user
func (c *Client) ListenForMessages() {
	for {
		serverMsg, err := c.stream.Recv()
		if err != nil {
			log.Printf("Failed to receive message from server: %v", err)
			break
		}
		fmt.Printf("Server: %s\n", serverMsg.Message)

		// Check if the experiment has started
		if strings.Contains(serverMsg.Message, "Experiment started") {
			c.start <- struct{}{}
		} else {
			c.msg <- serverMsg.Message
		}
	}
}

func (c *Client) WaitForStart() {
	<-c.start
}

func (c *Client) WaitForMessage() string {
	return <-c.msg
}

func (c *Client) SendGuess(guess int32) error {
	return c.stream.Send(&pb.ClientMessage{
		Username: c.username,
		Number:   guess,
	})
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

	// Gracefully handle system interrupts for cleanup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\nReceived interrupt signal, shutting down gracefully...")
		client.Close()
		os.Exit(0)
	}()

	// Listen for messages from the server in a separate goroutine
	go client.ListenForMessages()

	// Wait for the experiment to start
	fmt.Println("Waiting for the experiment to start...")
	client.WaitForStart()

	for {
		fmt.Print("Enter your guess (1-100): ")
		guessStr, _ := reader.ReadString('\n')
		guessStr = strings.TrimSpace(guessStr)

		guess, err := strconv.Atoi(guessStr)
		if err != nil || guess < 1 || guess > 100 {
			fmt.Println("Invalid input. Please enter a number between 1 and 100.")
			continue
		}

		err = client.SendGuess(int32(guess))
		if err != nil {
			log.Printf("Failed to send guess: %v", err)
			break
		}

		fmt.Println("Waiting for response...")
	}
}
