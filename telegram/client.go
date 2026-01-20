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
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// CloseInvoker is a pooled invoker that can be closed
type CloseInvoker interface {
	tg.Invoker
	Close() error
}

// Client wraps the gotd Telegram client
type Client struct {
	client      *telegram.Client
	api         *tg.Client
	pool        CloseInvoker // Connection pool for downloads
	pooledAPI   *tg.Client   // API client backed by the pool
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

	// Create logger - use Info level to reduce noise (Debug shows every request)
	zapCfg := zap.NewDevelopmentConfig()
	zapCfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	zapCfg.EncoderConfig.TimeKey = ""
	logger, _ := zapCfg.Build()

	// Use custom options to expose more control if needed
	opts := telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{
			Path: sessionPath,
		},
		UpdateHandler: &dispatcher,
		Logger:        logger,
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
func (c *Client) Run(ctx context.Context, botToken string, handler func(*Client) error) error {
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

		// Attempt to read session just to log DC (optional)
		data, err := os.ReadFile(c.sessionPath)
		if err == nil {
			var sess session.Data
			if err := json.Unmarshal(data, &sess); err == nil {
				log.Printf("ℹ️ Connected to DC %d", sess.DC)
			}
		}

		c.api = c.client.API()

		// Initialize Connection Pool for parallel downloads
		// Create pool - match or exceed your download manager's connection count
		const maxPoolConnections = 8
		pool, err := c.client.Pool(maxPoolConnections)
		if err != nil {
			log.Printf("⚠️ Failed to create connection pool: %v (falling back to single connection)", err)
			// Fallback: use standard API for downloads too
			c.pooledAPI = c.api
		} else {
			c.pool = pool
			c.pooledAPI = tg.NewClient(pool)
			log.Printf("✅ Connection pool created with max %d connections", maxPoolConnections)
		}

		// Run the handler with the full Client (provides access to API and PooledAPI)
		return handler(c)
	})
}

// API returns the underlying API client (single connection, for updates/messaging)
func (c *Client) API() *tg.Client {
	return c.api
}

// PooledAPI returns the pooled API client for parallel downloads
// Uses multiple TCP connections for better throughput
func (c *Client) PooledAPI() *tg.Client {
	if c.pooledAPI != nil {
		return c.pooledAPI
	}
	// Fallback to single connection API
	return c.api
}

// Close cleans up resources (call on shutdown)
func (c *Client) Close() error {
	if c.pool != nil {
		log.Println("🔌 Closing connection pool...")
		return c.pool.Close()
	}
	return nil
}
