package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gotd/td/tg"

	"tele-bot/config"
	"tele-bot/server"
	"tele-bot/storage"
	"tele-bot/telegram"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.APIID == 0 || cfg.APIHash == "" || cfg.BotToken == "" {
		log.Fatal("API_ID, API_HASH, and BOT_TOKEN are required. Please check your .env file")
	}

	log.Println("Starting Telegram Link Generator Service...")

	// Initialize storage
	store, err := storage.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	log.Println("Database initialized")

	// Create Telegram client with dispatcher
	client, dispatcher, err := telegram.NewClient(cfg.APIID, cfg.APIHash, cfg.BotToken, cfg.SessionPath)
	if err != nil {
		log.Fatalf("Failed to create Telegram client: %v", err)
	}
	log.Println("✅ Client and dispatcher created")

	// Set up context with cancellation
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start the application
	errChan := make(chan error, 2)

	// Run Telegram client
	go func() {
		err := client.Run(ctx, cfg.BotToken, func(api *tg.Client) error {
			log.Println("Telegram client connected")

			// Create downloader
			downloader := telegram.NewDownloader(api)

			// Create HTTP server
			httpServer := server.New(store, downloader, cfg.BaseURL)

			// Start HTTP server in a goroutine
			go func() {
				if err := httpServer.Start(cfg.HTTPPort); err != nil {
					errChan <- fmt.Errorf("HTTP server error: %w", err)
				}
			}()

			log.Printf("HTTP server listening on port %d", cfg.HTTPPort)
			log.Printf("Download links will be: %s/download/{id}", cfg.BaseURL)

			// Create message handler
			handler := telegram.NewHandler(api, store, cfg.BaseURL)

			// Register handlers with the dispatcher (the client is already listening!)
			if err := handler.Register(ctx, dispatcher); err != nil {
				return err
			}

			return nil
		})

		if err != nil && ctx.Err() == nil {
			errChan <- fmt.Errorf("Telegram client error: %w", err)
		}
	}()

	// Wait for error or shutdown signal
	select {
	case err := <-errChan:
		log.Printf("Error: %v", err)
		cancel()
	case <-ctx.Done():
		log.Println("Received shutdown signal")
	}

	log.Println("Service stopped")
}
