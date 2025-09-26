package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/gin-gonic/gin"
)

var casdoorClient *casdoorsdk.Client

func init() {
	casdoorClient = casdoorsdk.NewClient(
		os.Getenv("CASDOOR_ENDPOINT"),      // e.g., "http://localhost:8000"
		os.Getenv("CASDOOR_CLIENT_ID"),     // Your application's client ID
		os.Getenv("CASDOOR_CLIENT_SECRET"), // Your application's client secret
		os.Getenv("CASDOOR_CERTIFICATE"),   // JWT public key certificate
		os.Getenv("CASDOOR_ORGANIZATION"),  // Your organization name
		os.Getenv("CASDOOR_APPLICATION"),   // Your application name
	)
}

func main() {
	r := gin.Default()

	// Public routes
	r.GET("/", handleHome)
	r.GET("/login", handleLogin)
	r.GET("/callback", handleCallback)

	// Protected routes
	protected := r.Group("/")
	protected.Use(authMiddleware())
	{
		protected.GET("/profile", handleProfile)
		protected.GET("/logout", handleLogout)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func handleHome(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Welcome! Go to /login to authenticate",
		"status":  "unauthenticated",
	})
}

func handleLogin(c *gin.Context) {
	// Generate the authorization URL
	redirectURI := os.Getenv("CASDOOR_REDIRECT_URI") // e.g., "http://localhost:8080/callback"
	//state := "random-state-string"                   // In production, generate a secure random state

	if redirectURI == "" {
		fmt.Println("Empty redirect URI")
		redirectURI = "http://localhost:8080/callback"
	}

	authURL := casdoorClient.GetSigninUrl(redirectURI)

	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"message":  "Redirect user to this URL for authentication",
	})
}

func handleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Authorization code not provided",
		})
		return
	}

	// Validate state parameter (implement proper state validation in production)
	if state == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid state parameter",
		})
		return
	}

	token, err := casdoorClient.GetOAuthToken(code, state)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get token: %v", err),
		})
		return
	}
	// Parse the JWT token to get user information
	user, err := casdoorClient.ParseJwtToken(token.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to parse token: %v", err),
		})
		return
	}

	// In a real application, you would:
	// 1. Store the token securely (e.g., in session, database, or secure cookie)
	// 2. Create a session for the user
	// For this example, we'll just return the user info

	c.JSON(http.StatusOK, gin.H{
		"message":      "Authentication successful",
		"user":         user,
		"access_token": token.AccessToken,
		"expires_in":   token.Expiry,
	})
}

func handleProfile(c *gin.Context) {
	// Get user info from token (this would typically come from session/middleware)
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "No authorization token provided",
		})
		return
	}

	// Remove "Bearer " prefix if present
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	user, err := casdoorClient.ParseJwtToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

func handleLogout(c *gin.Context) {
	// In a real application, you would:
	// 1. Clear the user's session
	// 2. Optionally call Casdoor's logout endpoint
	// 3. Redirect to home page or login page

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// Middleware to protect routes
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "No authorization token provided",
			})
			c.Abort()
			return
		}

		// Remove "Bearer " prefix if present
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		// Validate the JWT token
		user, err := casdoorClient.ParseJwtToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Store user info in context for use in handlers
		c.Set("user", user)
		c.Next()
	}
}
