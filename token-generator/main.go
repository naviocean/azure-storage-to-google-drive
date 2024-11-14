package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "time"

    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
    "google.golang.org/api/drive/v3"
)

type TokenGenerator struct {
    logger        *log.Logger
    credPath      string
    tokenPath     string
    configuration *oauth2.Config
}

func NewTokenGenerator() (*TokenGenerator, error) {
    credPath := getEnvWithDefault("GOOGLE_CREDENTIALS_PATH", "credentials.json")
    tokenPath := getEnvWithDefault("GOOGLE_TOKEN_PATH", "token.json")

    // Create logger
    logger := log.New(os.Stdout, "[TOKEN-GENERATOR] ", log.LstdFlags)

    // Ensure credential file exists
    if _, err := os.Stat(credPath); os.IsNotExist(err) {
        return nil, fmt.Errorf("credentials file not found at: %s", credPath)
    }

    // Create directory for token if it doesn't exist
    tokenDir := filepath.Dir(tokenPath)
    if err := os.MkdirAll(tokenDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create token directory: %v", err)
    }

    return &TokenGenerator{
        logger:    logger,
        credPath:  credPath,
        tokenPath: tokenPath,
    }, nil
}

func (g *TokenGenerator) loadCredentials() error {
    b, err := os.ReadFile(g.credPath)
    if err != nil {
        return fmt.Errorf("unable to read credentials file: %v", err)
    }

    // Configure the oauth2 config with full drive scope for shared drives
    config, err := google.ConfigFromJSON(b, drive.DriveScope)
    if err != nil {
        return fmt.Errorf("unable to parse credentials: %v", err)
    }

    g.configuration = config
    return nil
}

func (g *TokenGenerator) generateToken() error {
    // Generate authorization URL
    authURL := g.configuration.AuthCodeURL("state-token",
        oauth2.AccessTypeOffline,
        oauth2.ApprovalForce) // Force approval to ensure refresh token

    // Print instructions
    fmt.Printf("\n=== Google Drive Authorization Required ===\n")
    fmt.Printf("\n1. Visit the following URL in your browser:\n\n%v\n", authURL)
    fmt.Printf("\n2. After authorization, copy the code from the browser.\n")
    fmt.Print("\nEnter the authorization code: ")

    // Get authorization code from user
    var authCode string
    if _, err := fmt.Scan(&authCode); err != nil {
        return fmt.Errorf("unable to read authorization code: %v", err)
    }

    // Exchange authorization code for token
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    token, err := g.configuration.Exchange(ctx, authCode)
    if err != nil {
        return fmt.Errorf("unable to retrieve token: %v", err)
    }

    // Verify token has refresh token
    if token.RefreshToken == "" {
        return fmt.Errorf("no refresh token received. Please revoke application access and try again")
    }

    return g.saveToken(token)
}

func (g *TokenGenerator) saveToken(token *oauth2.Token) error {
    g.logger.Printf("Saving token to: %s", g.tokenPath)

    f, err := os.OpenFile(g.tokenPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
    if err != nil {
        return fmt.Errorf("unable to cache oauth token: %v", err)
    }
    defer f.Close()

    // Save token with pretty print for readability
    encoder := json.NewEncoder(f)
    encoder.SetIndent("", "    ")
    if err := encoder.Encode(token); err != nil {
        return fmt.Errorf("unable to encode token: %v", err)
    }

    g.logger.Printf("Token successfully saved!")
    return nil
}

func (g *TokenGenerator) validateExistingToken() error {
    // Check if token file exists
    if _, err := os.Stat(g.tokenPath); os.IsNotExist(err) {
        return nil // Token doesn't exist, need to generate new one
    }

    // Read existing token
    data, err := os.ReadFile(g.tokenPath)
    if err != nil {
        return fmt.Errorf("unable to read token file: %v", err)
    }

    var token oauth2.Token
    if err := json.Unmarshal(data, &token); err != nil {
        return fmt.Errorf("invalid token format: %v", err)
    }

    // Check if token has refresh token
    if token.RefreshToken == "" {
        return fmt.Errorf("existing token has no refresh token")
    }

    // Try to refresh token
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    tokenSource := g.configuration.TokenSource(ctx, &token)
    newToken, err := tokenSource.Token()
    if err != nil {
        return fmt.Errorf("token refresh failed: %v", err)
    }

    // Save refreshed token
    if newToken.AccessToken != token.AccessToken {
        if err := g.saveToken(newToken); err != nil {
            return fmt.Errorf("unable to save refreshed token: %v", err)
        }
        g.logger.Printf("Token successfully refreshed!")
    } else {
        g.logger.Printf("Existing token is still valid!")
    }

    return nil
}

func getEnvWithDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func main() {
    generator, err := NewTokenGenerator()
    if err != nil {
        log.Fatalf("Failed to initialize token generator: %v", err)
    }

    generator.logger.Println("Starting Google Drive token generator...")

    // Load credentials
    if err := generator.loadCredentials(); err != nil {
        log.Fatalf("Failed to load credentials: %v", err)
    }

    // Validate existing token if any
    err = generator.validateExistingToken()
    if err != nil {
        generator.logger.Printf("Existing token validation failed: %v", err)
        generator.logger.Println("Generating new token...")

        if err := generator.generateToken(); err != nil {
            log.Fatalf("Failed to generate token: %v", err)
        }
    }
}