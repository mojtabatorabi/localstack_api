// lambda/file-processor/main.go

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	_ "github.com/lib/pq"
)

type FileProcessingRequest struct {
	FileID int    `json:"fileID"`
	S3Key  string `json:"s3Key"`
}

func main() {
	lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, sqsEvent events.SQSEvent) error {
	// Initialize AWS clients
	awsEndpoint := os.Getenv("AWS_ENDPOINT")
	if awsEndpoint == "" {
		awsEndpoint = "http://localstack:4566"
	}

	awsConfig := &aws.Config{
		Endpoint:    aws.String(awsEndpoint),
		Region:      aws.String("us-east-1"),
		Credentials: credentials.AnonymousCredentials,
	}
	awsSession := session.Must(session.NewSession(awsConfig))
	s3Client := s3.New(awsSession)

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
		var request FileProcessingRequest
		err := json.Unmarshal([]byte(message.Body), &request)
		if err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		log.Printf("Processing file ID: %d, S3 Key: %s", request.FileID, request.S3Key)

		// Get file metadata from S3
		result, err := s3Client.HeadObject(&s3.HeadObjectInput{
			Bucket: aws.String("file-uploads"),
			Key:    aws.String(request.S3Key),
		})

		if err != nil {
			log.Printf("Error getting file metadata: %v", err)
			continue
		}

		fileSize := *result.ContentLength
		contentType := *result.ContentType

		log.Printf("File size: %d bytes, Content type: %s", fileSize, contentType)

		// Update database to mark file as processed
		_, err = db.Exec("UPDATE files SET processed = true WHERE id = $1", request.FileID)
		if err != nil {
			log.Printf("Failed to update database: %v", err)
			continue
		}

		log.Printf("Successfully processed file ID: %d", request.FileID)
	}

	return nil
}
