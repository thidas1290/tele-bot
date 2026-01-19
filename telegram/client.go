package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

// Client wraps the gotd Telegram client
type Client struct {
	client      *telegram.Client
	api         *tg.Client
	apiID       int
	apiHash     string
	sessionPath string
}

// NewClient creates a new Telegram client with proper update handling
func NewClient(apiID int, apiHash, botToken, sessionDir string) (*Client, *tg.UpdateDispatcher, error) {
	// Create session directory if it doesn't exist
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return nil, nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	sessionPath := filepath.Join(sessionDir, "session.json")

	// Create the dispatcher BEFORE the client
	dispatcher := tg.NewUpdateDispatcher()
	log.Println("🔌 Created update dispatcher")

	// Use custom options to expose more control if needed
	opts := telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{
			Path: sessionPath,
		},
		UpdateHandler: &dispatcher,
	}

	client := telegram.NewClient(apiID, apiHash, opts)

	return &Client{
		client:      client,
		apiID:       apiID,
		apiHash:     apiHash,
		sessionPath: sessionPath,
	}, &dispatcher, nil
}

// Run starts the client and handles authentication
func (c *Client) Run(ctx context.Context, botToken string, handler func(*tg.Client) error) error {
	return c.client.Run(ctx, func(ctx context.Context) error {
		// Authenticate as bot
		status, err := c.client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("auth status check failed: %w", err)
		}

		if !status.Authorized {
			if _, err := c.client.Auth().Bot(ctx, botToken); err != nil {
				return fmt.Errorf("bot authentication failed: %w", err)
			}
			log.Println("Bot authenticated successfully")
		} else {
			log.Println("Already authenticated")
		}

		// Initialize Connection Pool (Disabled for now as it requires complex setup)
		// We will use the standard single-connection client.

		// Attempt to read session just to log DC (optional)
		data, err := os.ReadFile(c.sessionPath)
		if err == nil {
			var sess session.Data
			if err := json.Unmarshal(data, &sess); err == nil {
				log.Printf("ℹ️ Connected to DC %d", sess.DC)
			}
		}

		c.api = c.client.API()

		// Run the handler
		return handler(c.api)
	})
}

// API returns the underlying API client
func (c *Client) API() *tg.Client {
	return c.api
}
