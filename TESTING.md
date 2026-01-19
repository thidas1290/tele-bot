# Testing Guide

## ✅ What's Now Working

The bot can now:
1. ✅ Receive messages from users
2. ✅ Process file uploads (documents, photos)
3. ✅ Generate unique download links
4. ✅ Reply back to users with the link
5. ✅ Handle text messages with instructions

## Quick Test

### 1. Start the Service

```bash
./tele-bot
```

You should see:
```
Starting Telegram Link Generator Service...
Database initialized  
Telegram client connected
📡 Setting up message update listener...
✅ Update listener ready - send a file to your bot!
HTTP server listening on port 8080
```

### 2. Send a File to Your Bot

Open Telegram and:
1. Find your bot (search @YourBotName)
2. Send `/start` (optional)
3. Send any file (document, PDF, image)

### 3. Check Logs

You should see:
```
📩 Received message from user 123456789
✅ File uploaded: document.pdf -> http://localhost:8080/download/abc-123-def (Size: 2.3 MB)
```

### 4. Bot Replies

The bot will reply with:
```
✅ File uploaded successfully!

📁 Name: `document.pdf`
📊 Size: 2.3 MB

🔗 Download link:
http://localhost:8080/download/abc-123-def-456

Link valid for downloads
```

### 5. Download the File

```bash
curl -O http://localhost:8080/download/abc-123-def-456
```

Or open the link in your browser!

### 6. Test Range Requests

```bash
# Download first 1KB only
curl -H "Range: bytes=0-1023" -o partial.bin \
     http://localhost:8080/download/abc-123-def-456

# Resume download
wget -c http://localhost:8080/download/abc-123-def-456
```

## Expected Behavior

| Action | Expected Result |
|--------|----------------|
| Send text message | Bot replies: "👋 Send me any file..." |
| Send document | Bot generates link and replies |
| Send photo | Bot generates link and replies |
| Send video | Bot replies: "⚠️ Unsupported media type" |
| Download with Range | Returns 206 Partial Content |
| Download full file | Returns 200 OK |

## Troubleshooting

**Bot doesn't respond:**
- Check logs for errors
- Verify bot token is correct
- Ensure API_ID and API_HASH are set

**"Update error" in logs:**
- Normal if connection drops briefly
- The gap recovery will catch up automatically

**Download link doesn't work:**
- Check database: `sqlite3 data/metadata.db "SELECT * FROM files;"`
- Verify BASE_URL matches your actual URL

## What to Test

- [x] Text messages get a reply
- [x] Documents generate links
- [x] Photos generate links  
- [x] Download links work
- [x] Range requests work (wget -c)
- [ ] Large files (>20MB) - to verify we bypass Bot API limit
- [ ] Multiple files in sequence
- [ ] Concurrent downloads
