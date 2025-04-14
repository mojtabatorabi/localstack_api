// api/handlers/messages.go

package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"myapp/models"
)

type MessageRequest struct {
	Message string `json:"message" binding:"required"`
}

func SendMessage(awsSession *session.Session, db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req MessageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get the username from context
		username := c.MustGet("username").(string)

		// Get user ID from database
		var userID int
		err := db.Get(&userID, "SELECT id FROM users WHERE username = $1", username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
			return
		}

		// Insert message into database
		var messageID int
		err = db.QueryRow(
			"INSERT INTO messages (user_id, message, status) VALUES ($1, $2, 'pending') RETURNING id",
			userID, req.Message,
		).Scan(&messageID)

		if err != nil {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save message"})
			return
		}

		// Send to SQS
		sqsClient := sqs.New(awsSession)
		queueURL := "http://localhost:4566/000000000000/message-queue"

		messageBody, _ := json.Marshal(map[string]interface{}{
			"messageID": messageID,
			"userID":    userID,
			"content":   req.Message,
		})

		_, err = sqsClient.SendMessage(&sqs.SendMessageInput{
			QueueUrl:    aws.String(queueURL),
			MessageBody: aws.String(string(messageBody)),
		})

		if err != nil {
			log.Printf("SQS error: %v", err)
			// We'll still return success as the message is saved
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":   "Message sent successfully",
			"messageID": messageID,
		})
	}
}

func ListMessages(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.MustGet("username").(string)

		var messages []models.Message
		err := db.Select(&messages, `
			SELECT m.* FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE u.username = $1
			ORDER BY m.created_at DESC
		`, username)

		if err != nil {
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve messages"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"messages": messages})
	}
}

func GetMessage(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		messageID := c.Param("id")
		username := c.MustGet("username").(string)

		var message models.Message
		err := db.Get(&message, `
			SELECT m.* FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.id = $1 AND u.username = $2
		`, messageID, username)

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": message})
	}
}
