package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

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

// loadToken loads the token from a file
func loadToken() (*TokenResponse, error) {
	file, err := os.Open("strava_token.json")
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			log.Printf("Error closing file: %v", cerr)
		}
	}()

	var token TokenResponse
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}
