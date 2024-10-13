package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	pb "hogwarts_experiment/proto"

	"google.golang.org/grpc"
)

func main() {
	// Connect to the server
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	client := pb.NewExperimentServiceClient(conn)

	// Create a bidirectional stream for communication with the server
	stream, err := client.Connect(context.Background())
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	// Listen for server messages in a separate goroutine
	go func() {
		for {
			// Receive a message from the server
			serverMsg, err := stream.Recv()
			if err != nil {
				log.Fatalf("Failed to receive message from server: %v", err)
			}
			fmt.Printf("Server: %s\n", serverMsg.Message)
		}
	}()

	// Wait for user guesses in the main goroutine
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter your guess: ")
		guessStr, _ := reader.ReadString('\n')
		guess, err := strconv.Atoi(guessStr[:len(guessStr)-1]) // Convert string to int
		if err != nil {
			fmt.Println("Invalid input. Please enter a number.")
			continue
		}

		// Send the guess to the server
		err = stream.Send(&pb.ClientMessage{Number: int32(guess)})
		if err != nil {
			log.Fatalf("Failed to send guess: %v", err)
		}

		// Pause before sending the next guess
		time.Sleep(time.Second)
	}
}
