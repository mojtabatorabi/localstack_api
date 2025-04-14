// lambda/message-processor/main.go

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/lib/pq"
)

type MessageProcessingRequest struct {
	MessageID int    `json:"messageID"`
	UserID    int    `json:"userID"`
	Content   string `json:"content"`
}

func main() {
	lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, sqsEvent events.SQSEvent) error {
	// Connect to database
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "postgres"
	}

	dsn := fmt.Sprintf("host=%s port=5432 user=postgres password=postgres dbname=appdb sslmode=disable", dbHost)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		return err
	}
	defer db.Close()

	// Process each message
	for _, message := range sqsEvent.Records {
		var request MessageProcessingRequest
		err := json.Unmarshal([]byte(message.Body), &request)
		if err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		log.Printf("Processing message ID: %d from user: %d", request.MessageID, request.UserID)

		// Simulate some processing time
		time.Sleep(500 * time.Millisecond)

		// Update message status
		_, err = db.Exec(
			"UPDATE messages SET status = 'processed', processed_at = NOW() WHERE id = $1",
			request.MessageID,
		)

		if err != nil {
			log.Printf("Failed to update message status: %v", err)
			continue
		}

		log.Printf("Successfully processed message ID: %d", request.MessageID)
	}

	return nil
}
