// api/middleware/cognito.go

package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/gin-gonic/gin"
)

type CognitoConfig struct {
	UserPoolID string
	ClientID   string
	Region     string
	Session    *session.Session
}

// InitAWSSession creates a new AWS session with LocalStack configuration
func InitAWSSession() *session.Session {
	sess := session.Must(session.NewSession(&aws.Config{
		Region:   aws.String("us-east-1"),
		Endpoint: aws.String("http://localhost:9229"),
	}))
	return sess
}

type CognitoMiddleware struct {
	config        CognitoConfig
	cognitoClient *cognitoidentityprovider.CognitoIdentityProvider
}

func NewCognitoMiddleware(config CognitoConfig) *CognitoMiddleware {
	return &CognitoMiddleware{
		config: config,
		cognitoClient: cognitoidentityprovider.New(config.Session, &aws.Config{
			Endpoint: aws.String("http://localhost:9229"),
		}),
	}
}

func (m *CognitoMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set Cognito config in context
		c.Set("cognito_user_pool_id", m.config.UserPoolID)
		c.Set("cognito_client_id", m.config.ClientID)

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		// Check for Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
			return
		}

		token := parts[1]
		username, err := m.validateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		// Set username in context for later use
		c.Set("username", username)
		c.Next()
	}
}

func (m *CognitoMiddleware) validateToken(token string) (string, error) {
	// In a real environment, this would validate the JWT token
	// For LocalStack/development, we'll make a GetUser call to verify the token

	input := &cognitoidentityprovider.GetUserInput{
		AccessToken: aws.String(token),
	}

	result, err := m.cognitoClient.GetUser(input)
	if err != nil {
		return "", fmt.Errorf("invalid token: %v", err)
	}

	if result.Username == nil {
		return "", errors.New("username not found in token")
	}

	return *result.Username, nil
}
