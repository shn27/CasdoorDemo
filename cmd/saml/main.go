package main

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"

	saml2 "github.com/russellhaering/gosaml2"
	"github.com/russellhaering/gosaml2/types"
	dsig "github.com/russellhaering/goxmldsig"
)

// ============================================================================
// CONFIGURATION - Update these values to match your setup
// ============================================================================

const (
	// Casdoor Configuration
	CasdoorURL = "http://localhost:8000" // Your Casdoor server URL
	AppName    = "cloud-be"              // Application name in Casdoor (must match exactly!)

	// Your Application Configuration
	ServerPort = ":9000"                 // Port your app runs on
	BaseURL    = "http://localhost:9000" // Your app's base URL
)

// ============================================================================
// GLOBAL VARIABLES
// ============================================================================

var (
	// SAML Service Provider - handles all SAML authentication
	// This object validates SAML responses and generates SAML requests
	samlSP *saml2.SAMLServiceProvider

	// Simple in-memory session storage
	// In production: Use Redis, database, or encrypted cookies
	// Key: sessionID, Value: UserSession
	sessions = make(map[string]*UserSession)
)

// ============================================================================
// DATA STRUCTURES
// ============================================================================

// UserSession stores authenticated user information
// After successful SAML authentication, we create this session
type UserSession struct {
	UserID      string `json:"user_id"`      // Unique user identifier from Casdoor
	DisplayName string `json:"display_name"` // User's full name or display name
	UserName    string `json:"username"`     // Username
	Email       string `json:"email"`        // User's email address (if available)
}

// APIResponse is a standard response structure for all API endpoints
type APIResponse struct {
	Success bool        `json:"success"`         // Whether the request was successful
	Message string      `json:"message"`         // Human-readable message
	Data    interface{} `json:"data,omitempty"`  // Response data (optional)
	Error   string      `json:"error,omitempty"` // Error message (optional)
}

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const userContextKey contextKey = "user"

// ============================================================================
// MAIN FUNCTION
// ============================================================================

func main() {
	log.Println("🚀 Starting SAML-Only Golang API Application...")

	// Step 1: Initialize SAML Service Provider
	// This fetches metadata from Casdoor and configures SAML
	var err error
	samlSP, err = initializeSAML()
	if err != nil {
		log.Fatalf("❌ Failed to initialize SAML: %v", err)
	}

	// Step 2: Setup HTTP routes
	// All endpoints return JSON responses

	// Public routes - no authentication required
	http.HandleFunc("/", handleHome)                  // API info
	http.HandleFunc("/login", handleLogin)            // Initiates SAML login (redirects to Casdoor)
	http.HandleFunc("/saml/callback", handleCallback) // Receives SAML response from Casdoor

	// Protected routes - authentication required
	http.HandleFunc("/dashboard", authMiddleware(handleDashboard)) // Protected endpoint
	http.HandleFunc("/profile", authMiddleware(handleProfile))     // User profile
	http.HandleFunc("/logout", handleLogout)                       // Logout handler

	// Step 3: Start HTTP server
	log.Println("\n✅ SAML Configuration Successful!")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("📍 Server URL:       %s", BaseURL)
	log.Printf("📍 Casdoor URL:      %s", CasdoorURL)
	log.Printf("📍 Application Name: %s", AppName)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("🔗 API Info:         GET  %s/", BaseURL)
	log.Printf("🔐 Login:            GET  %s/login", BaseURL)
	log.Printf("📨 SAML Callback:    POST %s/saml/callback", BaseURL)
	log.Printf("📊 Dashboard:        GET  %s/dashboard", BaseURL)
	log.Printf("👤 Profile:          GET  %s/profile", BaseURL)
	log.Printf("🚪 Logout:           POST %s/logout", BaseURL)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("\n🎯 Server listening on %s\n", ServerPort)

	// Start the server
	if err := http.ListenAndServe(ServerPort, nil); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}

// ============================================================================
// SAML INITIALIZATION
// ============================================================================

// initializeSAML sets up the SAML Service Provider
// This function:
// 1. Fetches SAML metadata from Casdoor
// 2. Extracts X.509 certificates for signature verification
// 3. Configures the SAML Service Provider
func initializeSAML() (*saml2.SAMLServiceProvider, error) {
	log.Println("\n🔧 Initializing SAML Service Provider...")

	// ========================================================================
	// STEP 1: Fetch SAML Metadata from Casdoor
	// ========================================================================

	// Metadata URL format: {CASDOOR_URL}/api/saml/metadata?application=admin/{APP_NAME}
	// This endpoint returns XML containing:
	// - IdP's entity ID (issuer)
	// - SSO login URL
	// - X.509 certificates for signature verification
	// - Supported bindings (HTTP-POST, HTTP-Redirect)
	metadataURL := fmt.Sprintf("%s/api/saml/metadata?application=admin/%s&enablePostBinding=false", CasdoorURL, AppName)

	log.Printf("📥 Fetching metadata from: %s", metadataURL)

	// HTTP GET request to fetch metadata
	resp, err := http.Get(metadataURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SAML metadata: %w", err)
	}
	defer resp.Body.Close()

	// Check if request was successful
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("metadata endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read the XML metadata
	rawMetadata, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata response: %w", err)
	}

	log.Printf("✅ Metadata fetched successfully (%d bytes)", len(rawMetadata))

	// ========================================================================
	// STEP 2: Parse XML Metadata
	// ========================================================================

	// EntityDescriptor is the root element of SAML metadata
	// It contains all information about the Identity Provider (Casdoor)
	metadata := &types.EntityDescriptor{}
	if err := xml.Unmarshal(rawMetadata, metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata XML: %w", err)
	}

	log.Println("✅ Metadata XML parsed successfully")

	// ========================================================================
	// STEP 3: Extract and Store X.509 Certificates
	// ========================================================================

	// These certificates are used to verify digital signatures on SAML responses
	// This ensures the response actually came from Casdoor and wasn't tampered with
	certStore := dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{},
	}

	// Loop through all key descriptors in metadata
	// Usually there's at least one certificate for signing
	certificateCount := 0
	for _, keyDescriptor := range metadata.IDPSSODescriptor.KeyDescriptors {
		// Each key descriptor can contain multiple certificates
		for idx, xmlCert := range keyDescriptor.KeyInfo.X509Data.X509Certificates {
			// Validate certificate data exists
			if xmlCert.Data == "" {
				log.Printf("⚠️  Warning: Certificate %d is empty, skipping", idx)
				continue
			}

			// Decode base64-encoded certificate
			certData, err := base64.StdEncoding.DecodeString(xmlCert.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to decode certificate %d: %w", idx, err)
			}

			// Parse the X.509 certificate
			cert, err := x509.ParseCertificate(certData)
			if err != nil {
				return nil, fmt.Errorf("failed to parse certificate %d: %w", idx, err)
			}

			// Add certificate to our trust store
			certStore.Roots = append(certStore.Roots, cert)
			certificateCount++

			// Log certificate information for debugging
			log.Printf("✅ Certificate %d added:", certificateCount)
			log.Printf("   Subject: %s", cert.Subject.CommonName)
			log.Printf("   Issuer: %s", cert.Issuer.CommonName)
			log.Printf("   Valid Until: %s", cert.NotAfter.Format("2006-01-02"))
		}
	}

	if certificateCount == 0 {
		return nil, fmt.Errorf("no valid certificates found in metadata")
	}

	log.Printf("✅ Added %d certificate(s) to trust store", certificateCount)

	// ========================================================================
	// STEP 4: Configure SAML Service Provider
	// ========================================================================

	// Create SAML Service Provider configuration
	// This defines how our application behaves as a SAML Service Provider (SP)
	sp := &saml2.SAMLServiceProvider{
		// ====================================================================
		// Identity Provider (IdP) Configuration - Casdoor's information
		// ====================================================================

		// Where to send SAML authentication requests
		// This is Casdoor's login endpoint
		IdentityProviderSSOURL: metadata.IDPSSODescriptor.SingleSignOnServices[0].Location,

		// Casdoor's unique identifier
		// Used to verify the SAML response came from the right IdP
		IdentityProviderIssuer: metadata.EntityID,

		// ====================================================================
		// Service Provider (SP) Configuration - Our application's information
		// ====================================================================

		// Our application's unique identifier (Entity ID)
		// This tells Casdoor who we are
		ServiceProviderIssuer: BaseURL + "/saml/metadata",

		// Where Casdoor should send the SAML response after authentication
		// This is our callback URL
		AssertionConsumerServiceURL: BaseURL + "/saml/callback",

		// Expected audience in SAML response
		// Security measure: SAML response must be intended for us
		AudienceURI: BaseURL + "/saml/metadata",

		// ====================================================================
		// Security Configuration
		// ====================================================================

		// Sign our SAML requests (recommended for production)
		// This proves to Casdoor that the request came from us
		SignAuthnRequests: true,

		// Certificate store for verifying Casdoor's signatures
		// When Casdoor sends us a SAML response, we use these certs to verify it
		IDPCertificateStore: &certStore,

		// Our private key for signing requests
		// For testing: using random key generator
		// For production: use proper key management (load from file/vault)
		SPKeyStore: dsig.RandomKeyStoreForTest(),
	}

	// Log the configuration for verification
	log.Println("\n✅ SAML Service Provider Configured:")
	log.Printf("   IdP SSO URL:  %s", sp.IdentityProviderSSOURL)
	log.Printf("   IdP Issuer:   %s", sp.IdentityProviderIssuer)
	log.Printf("   SP Issuer:    %s", sp.ServiceProviderIssuer)
	log.Printf("   ACS URL:      %s", sp.AssertionConsumerServiceURL)
	log.Printf("   Audience URI: %s", sp.AudienceURI)

	return sp, nil
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

// handleHome returns API information and authentication status
func handleHome(w http.ResponseWriter, r *http.Request) {
	// Check if user has a valid session
	sessionID := getSessionCookie(r)
	user, authenticated := sessions[sessionID]

	var response APIResponse

	if authenticated {
		// User is authenticated
		response = APIResponse{
			Success: true,
			Message: "User is authenticated",
			Data: map[string]interface{}{
				"authenticated": true,
				"user":          user,
				"endpoints": map[string]string{
					"profile":   BaseURL + "/profile",
					"dashboard": BaseURL + "/dashboard",
					"logout":    BaseURL + "/logout",
				},
			},
		}
	} else {
		// User is not authenticated
		response = APIResponse{
			Success: true,
			Message: "Welcome to SAML API. Please authenticate.",
			Data: map[string]interface{}{
				"authenticated": false,
				"auth_url":      BaseURL + "/login",
				"info": map[string]string{
					"casdoor_url": CasdoorURL,
					"app_name":    AppName,
					"callback":    BaseURL + "/saml/callback",
				},
			},
		}
	}

	sendJSON(w, http.StatusOK, response)
}

// handleLogin initiates SAML authentication
// This creates a SAML AuthnRequest and redirects user to Casdoor
func handleLogin(w http.ResponseWriter, r *http.Request) {
	log.Println("\n🔐 SAML Login Flow Started")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Build SAML authentication URL
	// This generates a SAMLRequest (base64 encoded XML)
	// and creates a URL to redirect the user to Casdoor
	authURL, err := samlSP.BuildAuthURL("")
	if err != nil {
		log.Printf("❌ Failed to build authentication URL: %v", err)
		sendJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to build auth URL: %v", err),
		})
		return
	}
	fmt.Println("kakaki=========", authURL)

	// Log the generated URL (truncated for readability)
	if len(authURL) > 150 {
		log.Printf("📤 Generated SAML AuthnRequest URL: %s...", authURL[:150])
	} else {
		log.Printf("📤 Generated SAML AuthnRequest URL: %s", authURL)
	}
	log.Println("🔄 Redirecting user to Casdoor for authentication...")

	// Redirect user to Casdoor's SAML login page
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// handleCallback processes SAML response from Casdoor
// After user logs in at Casdoor, they're redirected here with a SAMLResponse
// This handler validates the response and creates a user session
func handleCallback(w http.ResponseWriter, r *http.Request) {
	log.Println("\n📨 SAML Callback Received")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Parse form data
	// SAML responses can come via GET or POST
	if err := r.ParseForm(); err != nil {
		log.Printf("❌ Failed to parse form data: %v", err)
		sendJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Bad request",
		})
		return
	}

	// Extract SAMLResponse parameter
	// This contains the base64-encoded SAML assertion
	samlResponse := r.FormValue("SAMLResponse")
	if samlResponse == "" {
		// Try query parameter (for GET binding)
		samlResponse = r.URL.Query().Get("SAMLResponse")
	}

	if samlResponse == "" {
		log.Println("❌ No SAMLResponse found in request")
		sendJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "No SAML response found",
		})
		return
	}

	log.Printf("📦 SAMLResponse received (%d characters)", len(samlResponse))
	log.Println("🔍 Validating SAML response...")

	// ========================================================================
	// VALIDATE SAML RESPONSE
	// ========================================================================

	// This function:
	// 1. Decodes the base64 SAMLResponse
	// 2. Parses the XML
	// 3. Verifies the digital signature using IdP's certificate
	// 4. Checks timestamps (NotBefore, NotOnOrAfter)
	// 5. Validates audience restriction
	// 6. Extracts user attributes (assertions)
	assertionInfo, err := samlSP.RetrieveAssertionInfo(samlResponse)
	if err != nil {
		log.Printf("❌ SAML validation failed: %v", err)
		sendJSON(w, http.StatusForbidden, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Authentication failed: %v", err),
		})
		return
	}

	log.Println("✅ SAML response validated successfully")

	// ========================================================================
	// CHECK WARNING FLAGS
	// ========================================================================

	// Check if SAML assertion has invalid time
	// This happens if:
	// - Current time is before NotBefore
	// - Current time is after NotOnOrAfter
	if assertionInfo.WarningInfo.InvalidTime {
		log.Println("❌ SAML assertion time validation failed")
		log.Println("   Possible causes:")
		log.Println("   - Token expired")
		log.Println("   - Clock skew between servers")
		log.Println("   - Token not yet valid (NotBefore in future)")
		sendJSON(w, http.StatusForbidden, APIResponse{
			Success: false,
			Error:   "Token expired or not yet valid",
		})
		return
	}

	// Check if audience matches
	// The SAML response must be intended for our application
	if assertionInfo.WarningInfo.NotInAudience {
		log.Println("❌ Audience validation failed")
		log.Printf("   Expected audience: %s", BaseURL+"/saml/metadata")
		sendJSON(w, http.StatusForbidden, APIResponse{
			Success: false,
			Error:   "Invalid audience",
		})
		return
	}

	log.Println("✅ All security checks passed")

	// ========================================================================
	// EXTRACT USER INFORMATION
	// ========================================================================

	// NameID is the primary user identifier
	// This is guaranteed to be present in SAML response
	userID := assertionInfo.NameID

	// Extract additional user attributes
	// These are optional and depend on what Casdoor sends
	// Common attributes: DisplayName, Name, Email, etc.
	displayName := assertionInfo.Values.Get("DisplayName")
	userName := assertionInfo.Values.Get("Name")
	email := assertionInfo.Values.Get("Email")

	// Log extracted user information
	log.Println("👤 User Information Extracted:")
	log.Printf("   User ID:      %s", userID)
	log.Printf("   Display Name: %s", displayName)
	log.Printf("   Username:     %s", userName)
	if email != "" {
		log.Printf("   Email:        %s", email)
	}

	// ========================================================================
	// CREATE USER SESSION
	// ========================================================================

	// Generate unique session ID
	sessionID := fmt.Sprintf("saml_session_%s", userID)

	// Store user information in session
	sessions[sessionID] = &UserSession{
		UserID:      userID,
		DisplayName: displayName,
		UserName:    userName,
		Email:       email,
	}

	log.Printf("✅ Session created: %s", sessionID)

	// Set session cookie
	// This cookie will be sent with subsequent requests
	// HttpOnly: prevents JavaScript access (XSS protection)
	// Secure: should be true in production (requires HTTPS)
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		MaxAge:   3600,  // 1 hour
	})

	log.Println("🍪 Session cookie set")
	log.Println("🎉 Authentication successful!")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Return success response with user info
	sendJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Authentication successful",
		Data: map[string]interface{}{
			"user":       sessions[sessionID],
			"session_id": sessionID,
			"next_steps": map[string]string{
				"dashboard": BaseURL + "/dashboard",
				"profile":   BaseURL + "/profile",
			},
		},
	})
}

// handleDashboard shows protected dashboard data
// Only accessible to authenticated users
func handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Get user from request context (set by authMiddleware)
	user := r.Context().Value(userContextKey).(*UserSession)

	log.Printf("📊 Dashboard accessed by: %s", user.DisplayName)

	sendJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Welcome to your dashboard",
		Data: map[string]interface{}{
			"user":          user,
			"authenticated": true,
			"dashboard_data": map[string]interface{}{
				"message": "This is protected content only visible to authenticated users",
				"stats": map[string]int{
					"login_count": 1,
					"api_calls":   5,
				},
			},
		},
	})
}

// handleProfile returns user profile information
// Only accessible to authenticated users
func handleProfile(w http.ResponseWriter, r *http.Request) {
	// Get user from request context (set by authMiddleware)
	user := r.Context().Value(userContextKey).(*UserSession)

	log.Printf("👤 Profile accessed by: %s", user.DisplayName)

	sendJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "User profile",
		Data: map[string]interface{}{
			"profile": user,
		},
	})
}

// handleLogout logs out the user
// Clears session and returns success message
func handleLogout(w http.ResponseWriter, r *http.Request) {
	log.Println("🚪 User logout initiated")

	// Get and delete session
	sessionID := getSessionCookie(r)
	if sessionID != "" {
		if user, exists := sessions[sessionID]; exists {
			log.Printf("✅ Logging out user: %s", user.DisplayName)
			delete(sessions, sessionID)
		}
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "session_id",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	sendJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Logged out successfully",
		Data: map[string]interface{}{
			"login_url": BaseURL + "/login",
		},
	})
}

// ============================================================================
// MIDDLEWARE
// ============================================================================

// authMiddleware protects routes by requiring authentication
// It checks for valid session before allowing access to protected routes
//
// This middleware:
// 1. Checks if session cookie exists
// 2. Validates the session is still valid
// 3. Loads user information
// 4. Passes user to the next handler via context
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("🔒 Auth middleware checking authentication...")

		// Try to get session from cookie
		sessionID := getSessionCookie(r)
		if sessionID == "" {
			log.Println("❌ No session cookie found")
			sendJSON(w, http.StatusUnauthorized, APIResponse{
				Success: false,
				Error:   "Authentication required",
				Data: map[string]interface{}{
					"login_url": BaseURL + "/login",
				},
			})
			return
		}

		// Check if session exists in our session store
		user, exists := sessions[sessionID]
		if !exists {
			log.Println("❌ Invalid or expired session ID")
			sendJSON(w, http.StatusUnauthorized, APIResponse{
				Success: false,
				Error:   "Invalid or expired session",
				Data: map[string]interface{}{
					"login_url": BaseURL + "/login",
				},
			})
			return
		}

		log.Printf("✅ Valid session found for: %s", user.DisplayName)

		// Add user to request context
		// The handler can access this using: r.Context().Value(userContextKey)
		ctx := context.WithValue(r.Context(), userContextKey, user)

		// Call the next handler with the updated context
		next(w, r.WithContext(ctx))
	}
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// getSessionCookie retrieves session ID from cookie
func getSessionCookie(r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// sendJSON sends a JSON response with proper headers
func sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("❌ Failed to encode JSON response: %v", err)
	}
}
