package telegram

import (
	"context"
	"fmt"
	"io"

	"github.com/gotd/td/tg"
)

// Downloader handles file downloads from Telegram
type Downloader struct {
	api *tg.Client
}

// NewDownloader creates a new file downloader
func NewDownloader(api *tg.Client) *Downloader {
	return &Downloader{
		api: api,
	}
}

// DownloadFile downloads a file from Telegram and writes it to the provided writer
// Supports offset and limit for partial downloads (Range requests)
func (d *Downloader) DownloadFile(ctx context.Context, fileID int64, accessHash int64, fileReference []byte, offset int64, limit int64, w io.Writer) error {
	// Build InputDocument
	inputDoc := &tg.InputDocument{
		ID:            fileID,
		AccessHash:    accessHash,
		FileReference: fileReference,
	}

	// Calculate chunk size (must be divisible by 4KB, using 128KB for compatibility)
	const chunkSize = 128 * 1024

	// Adjust offset for chunk alignment
	// Telegram upload.getFile often requires Offset to be a multiple of the requested Limit (chunkSize)
	// especially for larger chunks.

	// Request aligned offset <= requested offset
	alignedOffset := offset - (offset % chunkSize)
	skipBytes := offset - alignedOffset

	currentOffset := alignedOffset
	bytesSent := int64(0)

	for bytesSent < limit {
		// Calculate how much to request
		requestLimit := int64(chunkSize)

		// Create location
		loc := &tg.InputDocumentFileLocation{
			ID:            inputDoc.ID,
			AccessHash:    inputDoc.AccessHash,
			FileReference: inputDoc.FileReference,
			ThumbSize:     "",
		}

		// Use api.UploadGetFile directly
		// req := &tg.UploadGetFileRequest{
		// 	Offset:   currentOffset,
		// 	Limit:    int(requestLimit),
		// 	Location: loc,
		// }

		// Note: upload.getFile may return less than limited if end of file.
		// However, gotd auto-generated code might be tricky.
		// Let's use the generated method: d.api.UploadGetFile(ctx, req)

		req := &tg.UploadGetFileRequest{
			Location: loc,
			Offset:   currentOffset,
			Limit:    int(requestLimit),
		}

		res, err := d.api.UploadGetFile(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to get file chunk at offset %d: %w", currentOffset, err)
		}

		var chunk []byte
		switch r := res.(type) {
		case *tg.UploadFile:
			chunk = r.Bytes
		case *tg.UploadFileCDNRedirect:
			return fmt.Errorf("CDN redirect not supported")
		default:
			return fmt.Errorf("unexpected response type: %T", res)
		}

		if len(chunk) == 0 {
			break // EOF
		}

		// If we need to skip bytes (alignment)
		dataToWrite := chunk
		if skipBytes > 0 {
			if int64(len(dataToWrite)) > skipBytes {
				dataToWrite = dataToWrite[skipBytes:]
				skipBytes = 0
			} else {
				skipBytes -= int64(len(dataToWrite))
				dataToWrite = nil
			}
		}

		// Limit to requested size
		remaining := limit - bytesSent
		if int64(len(dataToWrite)) > remaining {
			dataToWrite = dataToWrite[:remaining]
		}

		if len(dataToWrite) > 0 {
			n, err := w.Write(dataToWrite)
			if err != nil {
				return err // Client disconnected
			}
			bytesSent += int64(n)
		}

		currentOffset += int64(len(chunk))

		// If we got less than requested and we didn't limit it ourselves, likely EOF?
		// Telegram can return exact requested amount if available.
		if int64(len(chunk)) < requestLimit {
			// Check if we reached file end?
			// Actually we just continue until limit satisfied or chunk empty
		}

		// Check context again
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	return nil
}
