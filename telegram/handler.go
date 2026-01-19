package telegram

import (
	"context"
	"fmt"
	"log"
	"mime"

	"github.com/google/uuid"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"

	"tele-bot/storage"
)

// Handler processes incoming Telegram messages
type Handler struct {
	storage *storage.Storage
	baseURL string
	api     *tg.Client
	sender  *message.Sender
}

// NewHandler creates a new message handler
func NewHandler(api *tg.Client, storage *storage.Storage, baseURL string) *Handler {
	return &Handler{
		storage: storage,
		baseURL: baseURL,
		api:     api,
		sender:  message.NewSender(api),
	}
}

// Start registers message handlers with the pre-created dispatcher
func (h *Handler) Register(ctx context.Context, dispatcher *tg.UpdateDispatcher) error {
	log.Println("📡 Registering message handlers...")

	// Register handler with the EXISTING dispatcher (wired to client)
	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		log.Println("🔔 OnNewMessage triggered!")
		msg, ok := u.Message.(*tg.Message)
		if !ok {
			log.Println("⚠️  Message is not *tg.Message type")
			return nil
		}

		log.Printf("📩 Received message from user %d, text: %s", msg.PeerID, msg.Message)

		// Check for /start command
		if msg.Message == "/start" {
			log.Println("🚀 Received /start command")
			peer := h.getPeerFromMessage(msg)
			if peer != nil {
				_, err := h.sender.To(peer).Text(ctx,
					"🎉 *Welcome to File Link Generator Bot!*\n\n"+
						"Send me any file and I'll generate a download link for you.\n\n"+
						"Features:\n"+
						"📁 Documents, PDFs\n"+
						"🖼 Photos\n"+
						"🔗 HTTP Range support for resumable downloads")
				if err != nil {
					log.Printf("❌ Failed to send /start response: %v", err)
				} else {
					log.Println("✅ Sent /start welcome message")
				}
				return err
			}
		}

		return h.ProcessMessage(ctx, msg, e)
	})
	log.Println("✅ Handlers registered - bot is now listening!")

	// Wait for context cancellation - the client handles updates automatically now
	<-ctx.Done()
	return ctx.Err()
}

// ProcessMessage handles incoming messages with file uploads
func (h *Handler) ProcessMessage(ctx context.Context, msg *tg.Message, entities tg.Entities) error {
	// Check if message contains media
	if msg.Media == nil {
		// If it's just a text message, reply with instructions
		if msg.Message != "" {
			// Use PeerID to get the sender
			from := msg.GetPeerID()
			_, err := h.sender.To(&tg.InputPeerSelf{}).Text(ctx,
				"👋 Send me any file and I'll generate a download link for you!")
			if err != nil {
				// Try alternative: send to chat ID
				peer := h.getPeerFromMessage(msg)
				if peer != nil {
					_, err = h.sender.To(peer).Text(ctx,
						"👋 Send me any file and I'll generate a download link for you!")
				}
			}
			log.Printf("Replied to text message from user %d", from)
			return err
		}
		return nil
	}

	// Process different media types
	var fileID int64
	var accessHash int64
	var fileReference []byte
	var fileName string
	var fileSize int64
	var mimeType string

	switch media := msg.Media.(type) {
	case *tg.MessageMediaDocument:
		doc, ok := media.Document.(*tg.Document)
		if !ok {
			return nil
		}

		fileID = doc.ID
		accessHash = doc.AccessHash
		fileReference = doc.FileReference
		fileSize = doc.Size
		mimeType = doc.MimeType

		doc.AsInputDocumentFileLocation()

		// Extract filename from attributes
		for _, attr := range doc.Attributes {
			if filenameAttr, ok := attr.(*tg.DocumentAttributeFilename); ok {
				fileName = filenameAttr.FileName
				break
			}
		}

		if fileName == "" {
			// Generate filename from extension
			exts, _ := mime.ExtensionsByType(mimeType)
			ext := ".bin"
			if len(exts) > 0 {
				ext = exts[0]
			}
			fileName = fmt.Sprintf("file_%d%s", fileID, ext)
		}

	case *tg.MessageMediaPhoto:
		// Handle photos
		photo, ok := media.Photo.(*tg.Photo)
		if !ok {
			return nil
		}

		fileID = photo.ID
		accessHash = photo.AccessHash
		fileReference = photo.FileReference
		fileSize = 0 // Photos don't have a single size
		fileName = fmt.Sprintf("photo_%d.jpg", photo.ID)
		mimeType = "image/jpeg"

		// For photos, we'd need to find the largest size
		// Simplified for now

	default:
		peer := h.getPeerFromMessage(msg)
		if peer != nil {
			_, err := h.sender.To(peer).Text(ctx,
				"⚠️ Unsupported media type. Please send documents or photos.")
			return err
		}
		return nil
	}

	// Generate unique link ID
	linkID := uuid.New().String()

	// Save metadata to database
	err := h.storage.SaveFile(linkID, fileID, accessHash, fileReference, fileName, fileSize, mimeType)
	if err != nil {
		log.Printf("❌ Failed to save file metadata: %v", err)
		peer := h.getPeerFromMessage(msg)
		if peer != nil {
			_, replyErr := h.sender.To(peer).Text(ctx,
				"❌ Failed to process file. Please try again.")
			return replyErr
		}
		return err
	}

	// Generate download link
	downloadLink := fmt.Sprintf("%s/download/%s", h.baseURL, linkID)

	// Log the upload
	log.Printf("✅ File uploaded: %s -> %s (Size: %s)", fileName, downloadLink, formatFileSize(fileSize))

	// Send reply with download link
	peer := h.getPeerFromMessage(msg)
	if peer != nil {
		_, err = h.sender.To(peer).Text(ctx, fmt.Sprintf(
			"✅ *File uploaded successfully!*\n\n"+
				"📁 Name: `%s`\n"+
				"📊 Size: %s\n\n"+
				"🔗 *Download link:*\n%s\n\n"+
				"_Link valid for downloads_",
			fileName,
			formatFileSize(fileSize),
			downloadLink,
		))

		if err != nil {
			log.Printf("⚠️  Failed to send reply: %v", err)
		}
	}

	return err
}

// getPeerFromMessage extracts the peer from a message for replying
func (h *Handler) getPeerFromMessage(msg *tg.Message) tg.InputPeerClass {
	peer := msg.GetPeerID()

	// For bot chats, the peer is usually the user who sent the message
	switch p := peer.(type) {
	case *tg.PeerUser:
		return &tg.InputPeerUser{
			UserID: p.UserID,
		}
	case *tg.PeerChat:
		return &tg.InputPeerChat{
			ChatID: p.ChatID,
		}
	case *tg.PeerChannel:
		return &tg.InputPeerChannel{
			ChannelID: p.ChannelID,
		}
	}

	return nil
}

// formatFileSize formats bytes into human-readable format
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
