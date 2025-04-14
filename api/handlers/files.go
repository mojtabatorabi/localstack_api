// api/handlers/files.go

package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"myapp/models"
)

func UploadFile(awsSession *session.Session, db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the username from the context (set by middleware)
		username := c.MustGet("username").(string)
		log.Printf("Attempting to upload file for user: %s", username)

		// Get user ID from database
		var userID int
		err := db.Get(&userID, "SELECT id FROM users WHERE email = $1", username)
		if err != nil {
			log.Printf("Database error looking up user %s: %v", username, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
			return
		}
		log.Printf("Found user ID: %d for username: %s", userID, username)

		// Get the file from the form data
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			log.Printf("Error getting file from form: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
			return
		}
		defer file.Close()

		log.Printf("Processing file: %s, size: %d", header.Filename, header.Size)

		// Generate a unique filename
		filename := filepath.Base(header.Filename)
		fileKey := fmt.Sprintf("%s/%s_%s", username, uuid.New().String(), filename)
		log.Printf("Generated S3 key: %s", fileKey)

		// Upload to S3
		s3Client := s3.New(awsSession, &aws.Config{
			Endpoint:         aws.String("http://localhost:4566"),
			DisableSSL:       aws.Bool(true),
			S3ForcePathStyle: aws.Bool(true),
		})
		_, err = s3Client.PutObject(&s3.PutObjectInput{
			Bucket:      aws.String("my-bucket"),
			Key:         aws.String(fileKey),
			Body:        file,
			ContentType: aws.String(header.Header.Get("Content-Type")),
		})
		if err != nil {
			log.Printf("Error uploading to S3: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file"})
			return
		}

		// Save file metadata to database
		var fileID int
		err = db.QueryRow(
			"INSERT INTO files (user_id, filename, s3_key, content_type, size) VALUES ($1, $2, $3, $4, $5) RETURNING id",
			userID,
			filename,
			fileKey,
			header.Header.Get("Content-Type"),
			header.Size,
		).Scan(&fileID)

		if err != nil {
			log.Printf("Error saving file metadata: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
			return
		}

		log.Printf("File uploaded successfully with ID: %d", fileID)
		c.JSON(http.StatusCreated, gin.H{
			"message": "File uploaded successfully",
			"file_id": fileID,
		})
	}
}

func ListFiles(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.MustGet("username").(string)
		log.Printf("Listing files for user: %s", username)

		var files []models.File
		err := db.Select(&files, `
			SELECT f.* FROM files f
			JOIN users u ON f.user_id = u.id
			WHERE u.email = $1
			ORDER BY f.uploaded_at DESC
		`, username)

		if err != nil {
			log.Printf("Error listing files: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list files"})
			return
		}

		c.JSON(http.StatusOK, files)
	}
}

func GetFile(awsSession *session.Session, db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		fileID := c.Param("id")
		username := c.MustGet("username").(string)
		log.Printf("Getting file %s for user: %s", fileID, username)

		var file models.File
		err := db.Get(&file, `
			SELECT f.* FROM files f
			JOIN users u ON f.user_id = u.id
			WHERE f.id = $1 AND u.email = $2
		`, fileID, username)

		if err != nil {
			log.Printf("Error getting file: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
			return
		}

		// Get file from S3
		s3Client := s3.New(awsSession, &aws.Config{
			Endpoint:         aws.String("http://localhost:4566"),
			DisableSSL:       aws.Bool(true),
			S3ForcePathStyle: aws.Bool(true),
		})
		obj, err := s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String("my-bucket"),
			Key:    aws.String(file.S3Key),
		})
		if err != nil {
			log.Printf("Error getting file from S3: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file from storage"})
			return
		}
		defer obj.Body.Close()

		// Set headers for file download
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", file.Filename))
		c.Header("Content-Type", file.ContentType)
		c.Header("Content-Length", fmt.Sprintf("%d", file.Size))

		// Stream file to client
		c.Stream(func(w io.Writer) bool {
			_, err := io.Copy(w, obj.Body)
			return err == nil
		})
	}
}

func DeleteFile(awsSession *session.Session, db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		fileID := c.Param("id")
		username := c.MustGet("username").(string)
		log.Printf("Deleting file %s for user: %s", fileID, username)

		var s3Key string
		err := db.Get(&s3Key, `
			SELECT f.s3_key FROM files f
			JOIN users u ON f.user_id = u.id
			WHERE f.id = $1 AND u.email = $2
		`, fileID, username)

		if err != nil {
			log.Printf("Error getting file for deletion: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
			return
		}

		// Delete from S3
		s3Client := s3.New(awsSession, &aws.Config{
			Endpoint:         aws.String("http://localhost:4566"),
			DisableSSL:       aws.Bool(true),
			S3ForcePathStyle: aws.Bool(true),
		})
		_, err = s3Client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String("my-bucket"),
			Key:    aws.String(s3Key),
		})
		if err != nil {
			log.Printf("Error deleting file from S3: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file from storage"})
			return
		}

		// Delete from database
		_, err = db.Exec("DELETE FROM files WHERE id = $1", fileID)
		if err != nil {
			log.Printf("Error deleting file from database: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file record"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
	}
}
