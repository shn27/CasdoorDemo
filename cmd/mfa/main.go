package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/gin-gonic/gin"
)

var casdoorClient *casdoorsdk.Client
var isTest bool = true

func init() {
	cert := os.Getenv("CASDOOR_CERTIFICATE")
	if isTest {
		cert = strings.ReplaceAll(cert, `\n`, "\n")
	}

	casdoorClient = casdoorsdk.NewClient(
		os.Getenv("CASDOOR_ENDPOINT"), // e.g., "http://localhost:8000"
		os.Getenv("CASDOOR_CLIENT_ID"),
		os.Getenv("CASDOOR_CLIENT_SECRET"),
		cert,
		os.Getenv("CASDOOR_ORGANIZATION"), // e.g., "appscode"
		os.Getenv("CASDOOR_APPLICATION"),
	)
}

func main() {
	r := gin.Default()
	fmt.Println("kaka mfa=====")
	// Public routes
	r.GET("/", handleHome)
	r.GET("/login", handleLogin)
	r.GET("/token", handleTestToken)
	r.GET("/signup", handleSignup)
	r.GET("/callback", handleCallback)

	// Protected routes
	protected := r.Group("/")
	protected.Use(authMiddleware())
	{
		protected.GET("/profile", handleProfile)
		protected.GET("/logout", handleLogout)

		// MFA routes
		protected.POST("/mfa/initiate", handleMfaInitiate)
		protected.POST("/mfa/verify", handleMfaVerify)
		protected.POST("/mfa/enable", handleMfaEnable)
		protected.POST("/mfa/set-preferred", handleMfaSetPreferred)
		protected.DELETE("/mfa/delete", handleMfaDelete1)
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
	redirectURI := os.Getenv("CASDOOR_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = "http://localhost:8080/callback"
	}

	authURL := casdoorClient.GetSigninUrl(redirectURI)

	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"message":  "Redirect user to this URL for authentication",
	})
}

func handleSignup(c *gin.Context) {
	redirectURI := os.Getenv("CASDOOR_REDIRECT_URI")

	signupURL := casdoorClient.GetSignupUrl(true, redirectURI)

	c.JSON(http.StatusOK, gin.H{
		"auth_url":     signupURL,
		"redirect_uri": redirectURI,
		"message":      "Redirect user to this URL for registration",
		"action":       "signup",
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

	user, err := casdoorClient.ParseJwtToken(token.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to parse token: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Authentication successful",
		"user":         user,
		"access_token": token.AccessToken,
		"expires_in":   token.Expiry,
	})
}

func handleProfile(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not found in context",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

func handleLogout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// MFA Handlers

// Step 1: Initiate MFA setup - Get QR code and secret
func handleMfaInitiate(c *gin.Context) {
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

	var req struct {
		MfaType string `json:"mfaType" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: mfaType is required",
		})
		return
	}

	// Validate MFA type
	if req.MfaType != "app" && req.MfaType != "sms" && req.MfaType != "email" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid mfaType. Must be 'app', 'sms', or 'email'",
		})
		return
	}

	log.Printf("Initiating MFA for user: %s, owner: %s, type: %s",
		user.User.Name, user.User.Owner, req.MfaType)

	// Call Casdoor SDK's Initiate method
	// This hits: POST /api/mfa/setup/initiate
	mfaResp, err := casdoorClient.Initiate(
		user.Owner,  // Organization name (e.g., "appscode")
		req.MfaType, // MFA type ("app", "sms", or "email")
		user.Name,   // Username (e.g., "sohan")
	)

	if err != nil {
		log.Printf("MFA initiation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to initiate MFA: %v", err),
		})
		return
	}

	if mfaResp.Status != "ok" {
		log.Printf("MFA initiation failed: %s", mfaResp.Msg)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  mfaResp.Msg,
			"status": mfaResp.Status,
		})
		return
	}

	// Return the setup information to the client
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "MFA initiated successfully. Scan the QR code with your authenticator app.",
		"data": gin.H{
			"secret":         mfaResp.Data.Secret,        // Secret key for manual entry
			"qr_code_url":    mfaResp.Data.URL,           // otpauth:// URL for QR code
			"recovery_codes": mfaResp.Data.RecoveryCodes, // Backup recovery codes
			"mfa_type":       mfaResp.Data.MfaType,       // Type of MFA
			"enabled":        mfaResp.Data.Enabled,       // Is it already enabled?
		},
	})
}

// Step 2: Verify MFA code (optional, for testing before enabling)
func handleMfaVerify(c *gin.Context) {
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

	var req struct {
		MfaType  string `json:"mfaType" binding:"required"`
		Secret   string `json:"secret" binding:"required"`
		Passcode string `json:"passcode" binding:"required"` // 6-digit code from authenticator
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: mfaType, secret and passcode are required",
		})
		return
	}

	// Validate MFA type
	if req.MfaType != "app" && req.MfaType != "sms" && req.MfaType != "email" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid mfaType. Must be 'app', 'sms', or 'email'",
		})
		return
	}

	log.Printf("Verifying MFA code for user: %s", user.User.Name)

	// Call Casdoor SDK's Verify method
	// This hits: POST /api/mfa/setup/verify
	verifyResp, err := casdoorClient.Verify(
		user.User.Owner,
		req.MfaType,
		user.User.Name,
		req.Secret,
		req.Passcode,
	)

	if err != nil {
		log.Printf("MFA verification error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to verify MFA: %v", err),
		})
		return
	}

	if verifyResp.Status != "ok" {
		log.Printf("MFA verification failed: %s", verifyResp.Msg)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   verifyResp.Msg,
			"message": "Invalid MFA code. Please try again.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "MFA code verified successfully. You can now enable MFA.",
	})
}

// Custom enable function that includes recovery codes (workaround for SDK bug)
func enableMfaWithRecoveryCodes(owner, mfaType, name, secret string, recoveryCodes []string, token string) error {
	// Build the request body
	reqData := map[string]interface{}{
		"owner":         owner,
		"mfaType":       mfaType,
		"name":          name,
		"secret":        secret,
		"recoveryCodes": recoveryCodes,
	}

	postBytes, err := json.Marshal(reqData)
	if err != nil {
		return err
	}

	// Build the URL
	url := fmt.Sprintf("%s/api/mfa/setup/enable", os.Getenv("CASDOOR_ENDPOINT"))

	// Create the request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(postBytes))
	if err != nil {
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &result); err != nil {
			// Response might not be JSON, but request succeeded
			log.Printf("Response is not JSON (but request succeeded): %s", string(body))
			return nil
		}

		// Check if there's an error in the response
		if status, ok := result["status"].(string); ok && status != "ok" {
			msg := "Unknown error"
			if m, ok := result["msg"].(string); ok {
				msg = m
			}
			return fmt.Errorf("%s", msg)
		}
	}

	return nil
}

// Step 3: Enable MFA permanently
func handleMfaEnable(c *gin.Context) {
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

	var req struct {
		MfaType       string   `json:"mfaType" binding:"required"`
		Secret        string   `json:"secret"`
		RecoveryCodes []string `json:"recoveryCodes" binding:"required"` // Required! From initiate response
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: mfaType, secret, and recoveryCodes are required",
		})
		return
	}

	// Validate MFA type
	if req.MfaType != "app" && req.MfaType != "sms" && req.MfaType != "email" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid mfaType. Must be 'app', 'sms', or 'email'",
		})
		return
	}

	// Validate recovery codes
	if len(req.RecoveryCodes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "recoveryCodes cannot be empty",
		})
		return
	}
	log.Printf("Enabling MFA for user: %s with %d recovery codes", user.User.Name, len(req.RecoveryCodes))
	_, err := casdoorClient.Enable(user.User.Owner, req.MfaType, user.User.Name, req.Secret, req.RecoveryCodes[0])

	if err != nil {
		log.Printf("Enable MFA error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to enabled MFA: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "MFA enabled successfully. You will be asked for a code on your next login.",
	})
}

// Step 4 (Optional): Set preferred MFA method if user has multiple
func handleMfaSetPreferred(c *gin.Context) {
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

	var req struct {
		MfaType string `json:"mfaType" binding:"required"`
		Secret  string `json:"secret"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: mfaType and secret are required",
		})
		return
	}

	// Validate MFA type
	if req.MfaType != "app" && req.MfaType != "sms" && req.MfaType != "email" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid mfaType. Must be 'app', 'sms', or 'email'",
		})
		return
	}

	log.Printf("Setting preferred MFA for user: %s", user.User.Name)

	// Call Casdoor SDK's SetPreferred method
	// This hits: POST /api/set-preferred-mfa
	err := casdoorClient.SetPreferred(
		user.User.Owner,
		req.MfaType,
		user.User.Name,
		req.Secret,
	)

	if err != nil {
		log.Printf("Set preferred MFA error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to set preferred MFA: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Preferred MFA method set successfully",
	})
}

// Step 5: Delete/Disable MFA
func handleMfaDelete(c *gin.Context) {
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

	// Get the token from Authorization header
	token := c.GetHeader("Authorization")
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	log.Printf("Deleting MFA for user: %s", user.User.Name)

	// Build the URL with query parameters
	url := fmt.Sprintf("%s/api/delete-mfa?owner=%s&name=%s",
		os.Getenv("CASDOOR_ENDPOINT"),
		user.User.Owner,
		user.User.Name,
	)

	// Create the request
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create request: %v", err),
		})
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("MFA delete request error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to delete MFA: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to read response: %v", err),
		})
		return
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		log.Printf("MFA delete failed with status %d: %s", resp.StatusCode, string(body))
		c.JSON(resp.StatusCode, gin.H{
			"error": fmt.Sprintf("Failed to delete MFA: %s", string(body)),
		})
		return
	}

	// Try to parse response (but don't fail if it's empty)
	var result map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &result); err != nil {
			// Response might be empty or not JSON, but request succeeded
			log.Printf("Response is not JSON (but request succeeded): %s", string(body))
		} else {
			// Check if there's an error in the response
			if status, ok := result["status"].(string); ok && status != "ok" {
				msg := "Unknown error"
				if m, ok := result["msg"].(string); ok {
					msg = m
				}
				c.JSON(http.StatusBadRequest, gin.H{
					"error": msg,
				})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "MFA disabled successfully",
	})
}

// Step 5: Delete/Disable MFA
func handleMfaDelete1(c *gin.Context) {
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

	log.Printf("Deleting MFA for user: %s", user.User.Name)

	// Call Casdoor SDK's Delete method
	// This hits: POST /api/delete-mfa
	err := casdoorClient.Delete(
		user.User.Owner,
		user.User.Name,
	)

	if err != nil {
		log.Printf("MFA delete error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to delete MFA: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "MFA disabled successfully",
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

func handleTestToken(c *gin.Context) {
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
