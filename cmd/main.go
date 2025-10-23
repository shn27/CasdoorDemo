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
	r.GET("/token", handleTestToken)
	r.GET("/login", handleLogin)
	r.GET("/signup", handleSignup)
	r.GET("/callback", handleCallback)

	// Protected routes
	protected := r.Group("/")
	protected.Use(authMiddleware())
	{
		protected.GET("/profile", handleProfile)
		protected.GET("/logout", handleLogout)
		protected.GET("/mfa/initiate", handleMfaInitiate)
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

func handleSignup(c *gin.Context) {
	// Generate the authorization URL for signup
	redirectURI := os.Getenv("CASDOOR_REDIRECT_URI") // e.g., "http://localhost:8080/callback"

	// Debug: Print configuration to verify setup
	fmt.Printf("Signup - Endpoint: %s\n", os.Getenv("CASDOOR_ENDPOINT"))
	fmt.Printf("Signup - Client ID: %s\n", os.Getenv("CASDOOR_CLIENT_ID"))
	fmt.Printf("Signup - Redirect URI: %s\n", redirectURI)

	// Use GetSignupUrl for registration
	signupURL := casdoorClient.GetSignupUrl(true, redirectURI)

	c.JSON(http.StatusOK, gin.H{
		"auth_url":     signupURL,
		"redirect_uri": redirectURI,
		"message":      "Redirect user to this URL for registration",
		"action":       "signup",
	})
}

func getToken(code, state string) (string, error) {
	token, err := casdoorClient.GetOAuthToken(code, state)
	if err != nil {
		fmt.Println("Error getting token")
		return "", err
	}
	// Parse the JWT token to get user information
	_, err = casdoorClient.ParseJwtToken(token.AccessToken)
	if err != nil {
		fmt.Println("Failed to parse token")
		return "", err
	}
	return token.AccessToken, nil
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
	user, _ := casdoorClient.ParseJwtToken(token.AccessToken)
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
		code := "6258dbfbc9ec08c8aed4"
		state := "cloud-be"
		token, _ := getToken(code, state)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "No authorization token provided yes",
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

// MFA Handlers

func handleMfaInitiate(c *gin.Context) {
	fmt.Println("================")
	// Get user from context (set by authMiddleware)
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	user, ok := userInterface.(*casdoorsdk.Claims)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid user data",
		})
		return
	}

	// Parse request body
	//var req struct {
	//	MfaType string `json:"mfaType" binding:"required"` // "app", "sms", or "email"
	//}
	//
	//if err := c.ShouldBindJSON(&req); err != nil {
	//	c.JSON(http.StatusBadRequest, gin.H{
	//		"error": "Invalid request: mfaType is required (app, sms, or email)",
	//	})
	//	return
	//}
	//
	//// Validate MFA type
	//if req.MfaType != "app" && req.MfaType != "sms" && req.MfaType != "email" {
	//	c.JSON(http.StatusBadRequest, gin.H{
	//		"error": "Invalid mfaType. Must be 'app', 'sms', or 'email'",
	//	})
	//	return
	//}

	// Initiate MFA setup
	mfaResp, err := casdoorClient.Initiate(
		user.User.Owner, // Organization name
		"email",         // MFA type
		user.User.Name,  // Username
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to initiate MFA: %v", err),
		})
		return
	}

	if mfaResp.Status != "ok" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": mfaResp.Msg,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "MFA initiated successfully",
		"data": gin.H{
			"secret":         mfaResp.Data.Secret,
			"qr_code_url":    mfaResp.Data.URL,
			"recovery_codes": mfaResp.Data.RecoveryCodes,
			"mfa_type":       mfaResp.Data.MfaType,
		},
	})
}

func handleTestToken(c *gin.Context) {
	fmt.Println("==============")
	token := c.Query("token")
	if token == "" {
		c.JSON(400, gin.H{"error": "token query param required"})
		return
	}

	user, err := casdoorClient.ParseJwtToken(token)
	if err != nil {
		c.JSON(401, gin.H{
			"error":   "Parse failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{"user": user})
}
