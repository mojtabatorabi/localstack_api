package middleware

import (
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/gin-gonic/gin"
)

// SimpleAuthMiddleware is a basic token-based authentication middleware
func SimpleAuthMiddleware(awsSession *session.Session) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		// Expecting format: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		token := parts[1]

		// Get user info from Cognito
		cognitoClient := cognitoidentityprovider.New(awsSession)
		userInfo, err := cognitoClient.GetUser(&cognitoidentityprovider.GetUserInput{
			AccessToken: aws.String(token),
		})

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Find the email attribute
		var email string
		for _, attr := range userInfo.UserAttributes {
			if *attr.Name == "email" {
				email = *attr.Value
				break
			}
		}

		if email == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Email not found in token"})
			c.Abort()
			return
		}

		// Set the email in the context
		c.Set("username", email)
		c.Next()
	}
}
