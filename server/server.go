package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gotd/td/tg"

	"tele-bot/storage"
	"tele-bot/telegram"
)

// Server handles HTTP requests for file downloads
type Server struct {
	storage *storage.Storage
	api     *tg.Client // Pooled API for downloads
	baseURL string
}

// New creates a new HTTP server
func New(storage *storage.Storage, api *tg.Client, baseURL string) *Server {
	return &Server{
		storage: storage,
		api:     api,
		baseURL: baseURL,
	}
}

// Start begins the HTTP server
func (s *Server) Start(port int) error {
	http.HandleFunc("/download/", s.handleDownload)
	http.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("HTTP server starting on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// handleHealth is a simple health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleDownload handles file download requests
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	// Extract link ID from URL
	path := strings.TrimPrefix(r.URL.Path, "/download/")
	linkID := strings.TrimSpace(path)

	if linkID == "" {
		http.Error(w, "Invalid link", http.StatusBadRequest)
		return
	}

	// Get file metadata from database
	meta, err := s.storage.GetFileByLink(linkID)
	if err != nil {
		log.Printf("Error getting file metadata: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if meta == nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Parse Range header
	rangeHeader := r.Header.Get("Range")
	httpRange, err := ParseRange(rangeHeader, meta.FileSize)
	if err != nil {
		http.Error(w, "Invalid range", http.StatusRequestedRangeNotSatisfiable)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", meta.FileSize))
		return
	}

	// Set response headers
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", meta.MimeType)

	// Set Content-Disposition to suggest filename
	disposition := fmt.Sprintf("attachment; filename=\"%s\"", meta.FileName)
	w.Header().Set("Content-Disposition", disposition)

	// Determine status code and set appropriate headers
	if rangeHeader != "" && (httpRange.Start != 0 || httpRange.End != meta.FileSize-1) {
		// Partial content
		w.Header().Set("Content-Range", httpRange.ContentRange(meta.FileSize))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", httpRange.Length))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		// Full content
		w.Header().Set("Content-Length", fmt.Sprintf("%d", meta.FileSize))
		w.WriteHeader(http.StatusOK)
	}

	// Stream file from Telegram using TelegramReader
	ctx := r.Context()
	log.Printf("📥 Download request: start=%d, end=%d, length=%d", httpRange.Start, httpRange.End, httpRange.Length)

	// Create a TelegramReader for the requested byte range
	reader := telegram.NewTelegramReader(
		ctx,
		s.api,
		meta.FileID,
		meta.AccessHash,
		meta.FileReference,
		httpRange.Start,
		httpRange.End,
	)
	defer reader.Close()

	// Stream to HTTP response
	_, err = io.Copy(w, reader)
	if err != nil {
		log.Printf("Error streaming file: %v", err)
		// Can't send error response as headers already sent
		return
	}
}

// GenerateDownloadLink creates a download URL for a file
func (s *Server) GenerateDownloadLink(linkID string) string {
	return fmt.Sprintf("%s/download/%s", s.baseURL, linkID)
}
