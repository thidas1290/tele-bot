# Telegram Link Generator Service

A Golang service that allows users to upload files via Telegram bot and download them via HTTP with support for Range requests and multipart downloads.

## Features

- 📤 Upload files via Telegram bot (supports files up to 2GB/4GB)
- 🔗 Generate unique download links for each file
- 📊 HTTP Range request support for resumable downloads
- 🚀 Pure Go implementation (no native dependencies)
- 💾 SQLite database for metadata storage
- 🔄 Streams files directly from Telegram (no local file storage)

## Prerequisites

- Go 1.21 or higher
- Telegram API credentials (API ID and API Hash from [my.telegram.org](https://my.telegram.org))
- Telegram Bot Token from [@BotFather](https://t.me/botfather)

## Setup

### 1. Get Telegram Credentials

1. Go to [my.telegram.org](https://my.telegram.org) and log in
2. Navigate to "API development tools"
3. Create a new application to get your **API ID** and **API Hash**
4. Open [@BotFather](https://t.me/botfather) on Telegram
5. Send `/newbot` and follow instructions to get your **Bot Token**

### 2. Configure Environment

Copy the example environment file and fill in your credentials:

```bash
cp .env.example .env
```

Edit `.env` with your credentials:

```env
API_ID=your_api_id
API_HASH=your_api_hash
BOT_TOKEN=your_bot_token
HTTP_PORT=8080
BASE_URL=http://localhost:8080
```

### 3. Install Dependencies

```bash
go mod tidy
```

### 4. Run the Service

```bash
go run main.go
```

## Usage

1. **Upload a file**: Send any document, video, or file to your Telegram bot
2. **Get download link**: The bot will respond with a unique HTTP download link
3. **Download**: Use the link in any browser or download manager

### Example with curl

```bash
# Full download
curl -O http://localhost:8080/download/{link_id}

# Partial download (Range request)
curl -H "Range: bytes=0-1023" http://localhost:8080/download/{link_id}

# Resume download
curl -C - -O http://localhost:8080/download/{link_id}
```

### Example with wget

```bash
# Download with resume support
wget -c http://localhost:8080/download/{link_id}
```

## API Endpoints

### `GET /download/{link_id}`

Download a file by its unique link ID.

**Headers:**
- `Range` (optional): Specify byte range for partial download
  - Format: `bytes=start-end`
  - Example: `bytes=0-1023` (first 1KB)
  - Example: `bytes=1024-` (from byte 1024 to end)

**Response:**
- `200 OK`: Full file content
- `206 Partial Content`: Partial file content (when Range header is present)
- `404 Not Found`: File not found
- `416 Range Not Satisfiable`: Invalid range

### `GET /health`

Health check endpoint.

**Response:**
- `200 OK`: Service is running

## Architecture

```
┌─────────────┐         ┌──────────────┐
│   Telegram  │────────▶│  gotd Client │
│    Users    │         │   (MTProto)  │
└─────────────┘         └──────┬───────┘
                               │
                               ▼
                     ┌─────────────────┐
                     │  Message Handler│
                     │  - File Upload  │
                     │  - Link Gen     │
                     └────────┬────────┘
                              │
                              ▼
                     ┌─────────────────┐
                     │  SQLite Storage │
                     │  - File Metadata│
                     └─────────────────┘
                              │
                              │
┌─────────────┐         ┌────▼─────────┐
│HTTP Clients │────────▶│ HTTP Server  │
│  (Browser,  │         │ - Range Req  │
│   wget,     │◀────────│ - Streaming  │
│   curl)     │         └──────┬───────┘
└─────────────┘                │
                               ▼
                     ┌─────────────────┐
                     │   Downloader    │
                     │  (Stream from   │
                     │   Telegram)     │
                     └─────────────────┘
```

## Development

### Build

```bash
go build -o tele-bot
```

### Run Tests

```bash
go test ./...
```

## Deployment

### Docker (Coming Soon)

A Dockerfile will be provided for easy deployment.

### Systemd Service

Create `/etc/systemd/system/tele-bot.service`:

```ini
[Unit]
Description=Telegram Link Generator Service
After=network.target

[Service]
Type=simple
User=youruser
WorkingDirectory=/path/to/tele-bot
Environment="API_ID=your_api_id"
Environment="API_HASH=your_api_hash"
Environment="BOT_TOKEN=your_bot_token"
ExecStart=/path/to/tele-bot/tele-bot
Restart=always

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable tele-bot
sudo systemctl start tele-bot
```

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
