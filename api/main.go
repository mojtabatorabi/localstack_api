// api/main.go

package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"myapp/handlers"
	"myapp/middleware"
)

func main() {
	// Load environment variables from .env file
	envPath := filepath.Join("..", ".env")
	if err := godotenv.Load(envPath); err != nil {
		log.Printf("Warning: Error loading .env file from %s: %v", envPath, err)
		// Try loading from current directory
		if err := godotenv.Load(); err != nil {
			log.Printf("Warning: Error loading .env file: %v", err)
		}
	}

	// Initialize AWS session for LocalStack
	awsEndpoint := os.Getenv("COGNITO_IDP_URL")
	if awsEndpoint == "" {
		awsEndpoint = "http://localhost:9229"
	}
	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion == "" {
		awsRegion = "us-east-1"
	}

	log.Printf("Using Cognito endpoint: %s", awsEndpoint)
	log.Printf("Using AWS region: %s", awsRegion)

	awsConfig := &aws.Config{
		Endpoint:    aws.String(awsEndpoint),
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	}

	awsSession, err := session.NewSession(awsConfig)
	if err != nil {
		log.Fatalf("Failed to create AWS session: %v", err)
	}

	// Create S3 bucket if it doesn't exist
	s3Client := s3.New(awsSession, &aws.Config{
		Endpoint:         aws.String("http://localhost:4566"),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
	})

	_, err = s3Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String("my-bucket"),
	})
	if err != nil {
		// If the bucket already exists, that's fine
		if !strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") {
			log.Printf("Warning: Failed to create S3 bucket: %v", err)
		}
	}

	// Initialize Cognito configuration
	cognitoConfig := middleware.CognitoConfig{
		UserPoolID: os.Getenv("USER_POOL_ID"),
		ClientID:   os.Getenv("CLIENT_ID"),
		Session:    awsSession,
	}

	// Connect to PostgreSQL
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	// Log environment variables (without password)
	log.Printf("Database configuration:")
	log.Printf("  Host: %s", dbHost)
	log.Printf("  Port: %s", dbPort)
	log.Printf("  User: %s", dbUser)
	log.Printf("  Database: %s", dbName)

	// URL encode the password to handle special characters
	encodedPassword := url.QueryEscape(dbPassword)
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, encodedPassword, dbHost, dbPort, dbName)

	log.Printf("Connecting to database with DSN: postgres://%s:****@%s:%s/%s?sslmode=disable",
		dbUser, dbHost, dbPort, dbName)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create users table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			cognito_username VARCHAR(255) UNIQUE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create users table: %v", err)
	}

	// Create files table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS files (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			filename VARCHAR(255) NOT NULL,
			s3_key VARCHAR(255) NOT NULL,
			content_type VARCHAR(255),
			size BIGINT,
			uploaded_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create files table: %v", err)
	}

	// Create Gin router
	router := gin.Default()

	// Public routes
	router.GET("/health", handlers.HealthCheck)

	// Auth routes
	auth := router.Group("/api/auth")
	{
		auth.POST("/register", handlers.Register(awsSession, db, &cognitoConfig))
		auth.POST("/login", handlers.Login(awsSession, &cognitoConfig))
	}

	// Protected routes
	authorized := router.Group("/api")
	authorized.Use(middleware.SimpleAuthMiddleware(awsSession))
	{
		// File routes
		authorized.POST("/files", handlers.UploadFile(awsSession, db))
		authorized.GET("/files", handlers.ListFiles(db))
		authorized.GET("/files/:id", handlers.GetFile(awsSession, db))
		authorized.DELETE("/files/:id", handlers.DeleteFile(awsSession, db))
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
