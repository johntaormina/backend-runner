package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// StravaConfig holds your Strava API credentials
type StravaConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// TokenResponse represents the response from Strava token endpoint
type TokenResponse struct {
	AccessToken  string         `json:"access_token"`
	RefreshToken string         `json:"refresh_token"`
	ExpiresAt    int64          `json:"expires_at"`
	ExpiresIn    int            `json:"expires_in"`
	TokenType    string         `json:"token_type"`
	Athlete      map[string]any `json:"athlete"`
}

// StravaClient is a client for interacting with the Strava API
type StravaClient struct {
	Config StravaConfig
	Token  *TokenResponse
}

func main() {
	// Example usage of the Strava client
	client, err := NewStravaClient()
	if err != nil {
		log.Fatalf("Error creating Strava client: %v", err)
	}

	// After authentication, fetch athlete activities
	activities, err := client.GetActivities(10)
	if err != nil {
		log.Fatalf("Error fetching activities: %v", err)
	}

	// Print activities
	fmt.Println("Your recent activities:")
	for i, activity := range activities {
		// Safely handle different field types
		name := getStringValue(activity, "name")

		// Handle date which could be a string or timestamp
		var dateStr string
		if startDateStr, ok := activity["start_date_local"].(string); ok {
			// If it's a string, parse it
			if t, err := time.Parse(time.RFC3339, startDateStr); err == nil {
				dateStr = t.Format("2006-01-02")
			} else {
				dateStr = startDateStr
			}
		} else if startDateFloat, ok := activity["start_date_local"].(float64); ok {
			// If it's a float (timestamp), convert to time
			dateStr = time.Unix(int64(startDateFloat), 0).Format("2006-01-02")
		} else {
			dateStr = "Unknown date"
		}

		// Handle distance
		distance := 0.0
		if distFloat, ok := activity["distance"].(float64); ok {
			distance = distFloat / 1000
		}

		fmt.Printf("%d. %s (%s) - %.2f km\n", i+1, name, dateStr, distance)
	}

}

// NewStravaClient creates a new Strava client
func NewStravaClient() (*StravaClient, error) {
	// Load config from environment variables (in production)
	// For this example, we'll hardcode them
	config := StravaConfig{
		ClientID:     
		ClientSecret: 
		RedirectURI:  "http://localhost:8080/callback",
	}

	client := &StravaClient{
		Config: config,
	}

	// Try to load existing token
	token, err := loadToken()
	if err == nil {
		client.Token = token

		// Check if token is expired and refresh if needed
		if time.Now().Unix() > token.ExpiresAt {
			newToken, err := client.refreshToken(token.RefreshToken)
			if err != nil {
				return nil, fmt.Errorf("failed to refresh token: %v", err)
			}
			client.Token = newToken
			saveToken(newToken)
		}
		return client, nil
	}

	// No valid token exists, start OAuth flow
	fmt.Println("No valid token found. Starting OAuth flow...")

	// Create channel to wait for callback
	authCompleted := make(chan *TokenResponse)

	// Start HTTP server to handle callback
	server := startAuthServer(config, authCompleted)

	// Generate and print authorization URL
	authURL := fmt.Sprintf(
		"https://www.strava.com/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code&scope=read,activity:read_all",
		config.ClientID,
		url.QueryEscape(config.RedirectURI),
	)

	fmt.Printf("Open this URL in your browser to authorize the application:\n%s\n", authURL)

	// Wait for authentication to complete
	token = <-authCompleted

	// Shutdown server
	if err := server.Shutdown(nil); err != nil {
		log.Printf("Error shutting down server: %v", err)
	}

	client.Token = token
	return client, nil
}

// startAuthServer starts an HTTP server to handle the OAuth callback
func startAuthServer(config StravaConfig, authCompleted chan<- *TokenResponse) *http.Server {
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Extract the authorization code from the request
		code := r.URL.Query().Get("code")
		if code == "" {
			error := r.URL.Query().Get("error")
			fmt.Fprintf(w, "Error: %s", error)
			authCompleted <- nil
			return
		}

		// Exchange the code for an access token
		token, err := exchangeCodeForToken(code, config)
		if err != nil {
			fmt.Fprintf(w, "Error exchanging code for token: %v", err)
			authCompleted <- nil
			return
		}

		// Display success message
		fmt.Fprintf(w, "Authentication successful! You can close this window.")

		// Save token to file
		saveToken(token)

		// Send token through channel
		authCompleted <- token
	})

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return server
}

// exchangeCodeForToken exchanges the authorization code for an access token
func exchangeCodeForToken(code string, config StravaConfig) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", config.ClientID)
	data.Set("client_secret", config.ClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")

	resp, err := http.Post(
		"https://www.strava.com/oauth/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to send token request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %s", body)
	}

	var tokenResponse TokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %v", err)
	}

	return &tokenResponse, nil
}

// refreshToken refreshes an expired access token
func (c *StravaClient) refreshToken(refreshToken string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", c.Config.ClientID)
	data.Set("client_secret", c.Config.ClientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	resp, err := http.Post(
		"https://www.strava.com/oauth/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to send refresh token request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed: %s", body)
	}

	var tokenResponse TokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %v", err)
	}

	return &tokenResponse, nil
}

// saveToken saves the token to a file
func saveToken(token *TokenResponse) error {
	file, err := os.Create("strava_token.json")
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(token)
}

// loadToken loads the token from a file
func loadToken() (*TokenResponse, error) {
	file, err := os.Open("strava_token.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var token TokenResponse
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}

// GetActivities fetches activities from the Strava API
func (c *StravaClient) GetActivities(limit int) ([]map[string]any, error) {
	// Ensure we have a valid token
	if c.Token == nil {
		return nil, fmt.Errorf("no valid token")
	}

	// Build request
	req, err := http.NewRequest("GET", "https://www.strava.com/api/v3/athlete/activities", nil)
	if err != nil {
		return nil, err
	}

	// Add query parameters
	q := req.URL.Query()
	q.Add("per_page", fmt.Sprintf("%d", limit))
	req.URL.RawQuery = q.Encode()

	// Add authorization header
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	// Make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed: %s", body)
	}

	// Parse response
	var activities []map[string]any
	if err := json.Unmarshal(body, &activities); err != nil {
		return nil, err
	}

	return activities, nil
}

// getStringValue safely extracts a string value from a map
func getStringValue(m map[string]any, key string) string {
	if val, ok := m[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
		// Try to convert to string
		return fmt.Sprintf("%v", val)
	}
	return "Unknown"
}
