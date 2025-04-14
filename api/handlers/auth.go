// api/handlers/auth.go

package handlers

import (
	"log"
	"net/http"

	"myapp/middleware"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func Register(awsSession *session.Session, db *sqlx.DB, config *middleware.CognitoConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required,min=8"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		log.Printf("Starting registration for email: %s", req.Email)

		// Create user in Cognito
		cognitoClient := cognitoidentityprovider.New(awsSession)
		createUserInput := &cognitoidentityprovider.AdminCreateUserInput{
			UserPoolId: aws.String(config.UserPoolID),
			Username:   aws.String(req.Email), // Use email as username
			UserAttributes: []*cognitoidentityprovider.AttributeType{
				{
					Name:  aws.String("email"),
					Value: aws.String(req.Email),
				},
				{
					Name:  aws.String("email_verified"),
					Value: aws.String("true"),
				},
			},
			MessageAction: aws.String("SUPPRESS"),
		}

		log.Printf("Creating user in Cognito with input: %+v", createUserInput)
		createUserOutput, err := cognitoClient.AdminCreateUser(createUserInput)
		if err != nil {
			log.Printf("Error creating user in Cognito: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user in Cognito"})
			return
		}

		log.Printf("User created in Cognito, setting password")
		// Set user password
		_, err = cognitoClient.AdminSetUserPassword(&cognitoidentityprovider.AdminSetUserPasswordInput{
			UserPoolId: aws.String(config.UserPoolID),
			Username:   aws.String(req.Email),
			Password:   aws.String(req.Password),
			Permanent:  aws.Bool(true),
		})
		if err != nil {
			log.Printf("Error setting user password: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set user password"})
			return
		}

		// Create user in local database
		log.Printf("Creating user in local database with email: %s, cognito_username: %s",
			req.Email, *createUserOutput.User.Username)

		var userID int
		err = db.QueryRow(
			`INSERT INTO users (email, cognito_username) 
			 VALUES ($1, $2)
			 ON CONFLICT (email) DO UPDATE 
			 SET cognito_username = EXCLUDED.cognito_username
			 RETURNING id`,
			req.Email,
			*createUserOutput.User.Username,
		).Scan(&userID)

		if err != nil {
			log.Printf("Error creating/updating user in database: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to create user in database",
				"details": err.Error(),
			})
			return
		}

		log.Printf("User created/updated successfully with ID: %d", userID)
		c.JSON(http.StatusCreated, gin.H{
			"message": "User registered successfully",
			"user_id": userID,
		})
	}
}

func Login(awsSession *session.Session, config *middleware.CognitoConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if awsSession == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "AWS session not initialized"})
			return
		}

		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		log.Printf("Attempting login for user: %s", req.Username)

		// Use email as username
		cognitoClient := cognitoidentityprovider.New(awsSession)
		authInput := &cognitoidentityprovider.InitiateAuthInput{
			AuthFlow: aws.String("USER_PASSWORD_AUTH"),
			ClientId: aws.String(config.ClientID),
			AuthParameters: map[string]*string{
				"USERNAME": aws.String(req.Username),
				"PASSWORD": aws.String(req.Password),
			},
		}

		log.Printf("Initiating auth with input: %+v", authInput)
		authResult, err := cognitoClient.InitiateAuth(authInput)
		if err != nil {
			log.Printf("Auth error: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		if authResult == nil || authResult.AuthenticationResult == nil {
			log.Printf("Auth result is nil")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
			return
		}

		if authResult.AuthenticationResult.AccessToken == nil {
			log.Printf("Access token is nil")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No access token received"})
			return
		}

		response := gin.H{
			"access_token": *authResult.AuthenticationResult.AccessToken,
		}

		if authResult.AuthenticationResult.RefreshToken != nil {
			response["refresh_token"] = *authResult.AuthenticationResult.RefreshToken
		}

		if authResult.AuthenticationResult.ExpiresIn != nil {
			response["expires_in"] = *authResult.AuthenticationResult.ExpiresIn
		}

		c.JSON(http.StatusOK, response)
	}
}
