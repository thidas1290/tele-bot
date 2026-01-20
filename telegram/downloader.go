package telegram

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/gotd/td/tg"
)

const (
	// ChunkSize must be divisible by 4KB.
	// Using 1MB (1024KB) to match TG-FileStreamBot reference implementation
	// for maximum throughput by minimizing API round-trips.
	ChunkSize = 1024 * 1024
)

// TelegramReader implements io.ReadCloser for streaming Telegram file downloads
type TelegramReader struct {
	ctx           context.Context
	api           *tg.Client
	location      tg.InputFileLocationClass
	start         int64 // Requested start byte
	end           int64 // Requested end byte (inclusive)
	next          func() ([]byte, error)
	buffer        []byte
	bufferPos     int64
	bytesRead     int64
	contentLength int64
}

// NewTelegramReader creates a new reader for downloading a byte range from Telegram
func NewTelegramReader(
	ctx context.Context,
	api *tg.Client,
	fileID int64,
	accessHash int64,
	fileReference []byte,
	start int64,
	end int64,
) io.ReadCloser {
	contentLength := end - start + 1

	location := &tg.InputDocumentFileLocation{
		ID:            fileID,
		AccessHash:    accessHash,
		FileReference: fileReference,
		ThumbSize:     "",
	}

	r := &TelegramReader{
		ctx:           ctx,
		api:           api,
		location:      location,
		start:         start,
		end:           end,
		contentLength: contentLength,
	}

	log.Printf("📥 TelegramReader: start=%d, end=%d, contentLength=%d", start, end, contentLength)

	r.next = r.partStream()
	return r
}

// Close implements io.Closer
func (r *TelegramReader) Close() error {
	return nil
}

// Read implements io.Reader - called repeatedly by io.CopyN
func (r *TelegramReader) Read(p []byte) (n int, err error) {
	// Check if we've read everything
	if r.bytesRead >= r.contentLength {
		return 0, io.EOF
	}

	// Check context cancellation
	if r.ctx.Err() != nil {
		return 0, r.ctx.Err()
	}

	// Need to fetch next chunk?
	if r.bufferPos >= int64(len(r.buffer)) {
		r.buffer, err = r.next()
		if err != nil {
			return 0, err
		}
		if len(r.buffer) == 0 {
			return 0, io.EOF
		}
		r.bufferPos = 0
	}

	// Copy from buffer to output
	n = copy(p, r.buffer[r.bufferPos:])
	r.bufferPos += int64(n)
	r.bytesRead += int64(n)

	return n, nil
}

// chunk fetches a single chunk from Telegram at the given offset
func (r *TelegramReader) chunk(offset int64, limit int64) ([]byte, error) {
	req := &tg.UploadGetFileRequest{
		Location: r.location,
		Offset:   offset,
		Limit:    int(limit),
	}

	res, err := r.api.UploadGetFile(r.ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk at offset %d: %w", offset, err)
	}

	switch result := res.(type) {
	case *tg.UploadFile:
		return result.Bytes, nil
	case *tg.UploadFileCDNRedirect:
		return nil, fmt.Errorf("CDN redirect not supported")
	default:
		return nil, fmt.Errorf("unexpected response type: %T", res)
	}
}

// partStream returns a closure that fetches and trims chunks sequentially
func (r *TelegramReader) partStream() func() ([]byte, error) {
	start := r.start
	end := r.end

	// Align offset to chunk boundary (round down)
	offset := start - (start % ChunkSize)

	// Calculate trimming for first and last chunks
	firstPartCut := start - offset
	lastPartCut := (end % ChunkSize) + 1
	partCount := int((end - offset + ChunkSize) / ChunkSize)
	currentPart := 1

	log.Printf("📊 partStream: offset=%d, firstCut=%d, lastCut=%d, parts=%d",
		offset, firstPartCut, lastPartCut, partCount)

	// Return a closure that fetches one chunk per call
	return func() ([]byte, error) {
		// Done fetching all parts?
		if currentPart > partCount {
			return []byte{}, nil
		}

		// Fetch chunk from Telegram
		chunk, err := r.chunk(offset, ChunkSize)
		if err != nil {
			return nil, err
		}

		// Empty chunk = EOF
		if len(chunk) == 0 {
			return chunk, nil
		}

		// Trim chunk to match requested byte range
		if partCount == 1 {
			// Single chunk: trim both start and end
			chunk = chunk[firstPartCut:lastPartCut]
		} else if currentPart == 1 {
			// First chunk: trim start
			chunk = chunk[firstPartCut:]
		} else if currentPart == partCount {
			// Last chunk: trim end
			chunk = chunk[:lastPartCut]
		}
		// Middle chunks: no trimming needed

		currentPart++
		offset += ChunkSize

		return chunk, nil
	}
}
