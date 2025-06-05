GitHub Mirror Bot

A comprehensive GitHub repository discovery and backup bot that automatically searches for repositories based on keywords, mirrors them locally, uploads to cloud storage, and sends detailed Discord notifications.

Features
🔍 Automated Repository Discovery: Search GitHub for repositories using customizable keywords and languages
📦 Smart Backup System: Clone or fork repositories with configurable filtering
☁️ Cloud Storage Integration: Automatic upload to Google Drive using rclone
🔔 Discord Notifications: Rich embed notifications with backup summaries
🚦 Rate Limiting: Built-in GitHub API rate limiting to prevent quota exhaustion
📊 Redis Queue Management: Efficient job queuing and duplicate prevention
⏰ Scheduled Operations: Configurable cron-based scheduling
🛡️ Secure Configuration: Environment variable support for sensitive data

Architecture

``
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   GitHub API    │───▶│  Mirror Bot     │───▶│   Redis Queue   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
                                ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Discord Webhook│◀───│  Local Storage  │───▶│  Cloud Storage  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
`

Prerequisites
Go 1.22 or higher
Redis server
Git
rclone (for cloud storage)
jq (for backup script)
curl

Installation
Clone the repository:
   `bash
   git clone https://github.com/NullMeDev/github-mirror-bot.git
   cd github-mirror-bot
   `
Install dependencies:
   `bash
   go mod tidy
   `
Set up environment variables:
   `bash
   cp .env.example .env
   # Edit .env with your actual values
   chmod 600 .env
   `
Configure the application:
   `bash
   cp config.yaml.example config.yaml
   # Edit config.yaml with your preferences
   `

Configuration

Environment Variables (.env)

`bash
GitHub Configuration
GITHUBTOKEN=ghpyouractualgithubtokenhere

Discord Configuration  
DISCORDWEBHOOKURL=https://discord.com/api/webhooks/YOURWEBHOOKID/YOURWEBHOOKTOKEN

Redis Configuration (optional)
REDISPASSWORD=yourredispasswordif_any

Optional: Override config file path
CONFIG_PATH=/path/to/your/config.yaml
`

Configuration File (config.yaml)

`yaml
github:
  tokenenv: GITHUBTOKEN

search:
  keywords: ["proxy", "bot", "keygen", "cracker", "scraper"]
  languages: ["go", "rust", "python", "c"]
  maxreposper_keyword: 50
  forkinsteadof_clone: true
  schedule: "0 /1   "  # Every hour

filter:
  maxinactivemonths: 12
  minstarsfor_stale: 120

storage:
  local_dir: /home/gitbackup/github-mirror
  remote: my2tb:github-mirror
  offloadafterminutes: 5

discord:
  webhookurlenv: DISCORDWEBHOOKURL
  enable_notifications: true
  batch_summary: true
  maxmessagelength: 1900

redis:
  address: "127.0.0.1:6379"
  passwordenv: REDISPASSWORD
  db: 0

logging:
  level: "info"
  file: "/var/log/github-mirror-bot.log"
`

Usage

Running the Bot

`bash
Run directly
go run cmd/main.go

Or build and run
go build -o github-mirror-bot cmd/main.go
./github-mirror-bot
`

Running the Backup Script

`bash
Make executable
chmod +x backup_github.sh

Run backup
./backup_github.sh
`

Docker Deployment

`dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o github-mirror-bot cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates git curl jq
WORKDIR /root/

COPY --from=builder /app/github-mirror-bot .
COPY config.yaml .

CMD ["./github-mirror-bot"]
`

Docker Compose

`yaml
version: '3.8'

services:
  github-mirror-bot:
    build: .
    env_file:
.env
    volumes:
./config.yaml:/root/config.yaml
./logs:/var/log
./data:/home/gitbackup/github-mirror
    depends_on:
redis
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    volumes:
redis_data:/data
    restart: unless-stopped

volumes:
  redis_data:
`

Discord Notifications

The bot sends rich Discord notifications with the following information:

Repository Discovery Notifications
Repository name and description
Star count and primary language
Backup status (success/failure)
Direct link to repository

Backup Summary Notifications
Total repositories found and processed
Success/failure counts
Processing duration
Cloud storage upload status
Detailed repository list

Example Discord Embed

`
🔍 New Repository Found: user/awesome-tool
⭐ Stars: 1,234
💻 Language: Go
📊 Status: ✅ Successfully queued
🔗 URL: View Repository
`

API Rate Limiting

The bot implements intelligent rate limiting:
GitHub API: 25 requests per minute (under the 30/min authenticated limit)
Token Bucket Algorithm: Prevents API quota exhaustion
Graceful Backoff: Automatic retry with exponential backoff

Filtering Logic

Repositories are filtered based on:
Activity: Recently updated repositories are prioritized
Popularity: Stale repositories need minimum star count
Duplicates: Redis-based deduplication prevents reprocessing
Keywords: Configurable search terms and languages

Monitoring and Logging
Structured Logging: Comprehensive logging with timestamps
Discord Alerts: Real-time notifications of operations
Error Tracking: Detailed error reporting and recovery
Metrics: Success/failure rates and processing times

Security Best Practices
✅ Environment variables for sensitive data
✅ No secrets in configuration files
✅ Secure file permissions (600) for .env
✅ Rate limiting to prevent abuse
✅ Input validation and sanitization
✅ Graceful error handling

Troubleshooting

Common Issues
GitHub API Rate Limit Exceeded:
   `bash
   # Check your rate limit status
   curl -H "Authorization: token $GITHUBTOKEN" https://api.github.com/ratelimit
   `
Redis Connection Failed:
   `bash
   # Test Redis connection
   redis-cli ping
   `
Discord Webhook Not Working:
   `bash
   # Test webhook manually
   curl -X POST -H "Content-Type: application/json" \
        -d '{"content":"Test message"}' \
        $DISCORDWEBHOOKURL
   `
rclone Upload Failed:
   `bash
   # Test rclone configuration
   rclone lsd my2tb:
   `

Debug Mode

Enable debug logging by setting the log level:

`yaml
logging:
  level: "debug"
`

Health Checks

The application provides health check endpoints:

`bash
Check if bot is running
ps aux | grep github-mirror-bot

Check Redis queue status
redis-cli llen mirror_jobs

Check log files
tail -f /var/log/github-mirror-bot.log
`

Development

Project Structure

`
github-mirror-bot/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go        # Configuration management
│   ├── search/
│   │   ├── search.go        # GitHub API search logic
│   │   ├── filter.go        # Repository filtering
│   │   └── queue.go         # Redis queue management
│   └── util/
│       ├── discord.go       # Discord webhook utilities
│       └── ratelimit.go     # Rate limiting implementation
├── backup_github.sh         # Backup script
├── config.yaml             # Configuration file
├── .env                    # Environment variables
├── go.mod                  # Go module definition
└── README.md              # This file
`

Adding New Features
New Search Filters: Modify internal/search/filter.go
Additional Notifications: Extend internal/util/discord.go
Custom Storage Backends: Implement new storage interfaces
Monitoring Endpoints: Add HTTP handlers in cmd/main.go

Testing

`bash
Run tests
go test ./...

Run with coverage
go test -cover ./...

Benchmark tests
go test -bench=. ./...
`

Contributing
Fork the repository
Create a feature branch (git checkout -b feature/amazing-feature)
Commit your changes (git commit -m 'Add amazing feature')
Push to the branch (git push origin feature/amazing-feature`)
Open a Pull Request

License

This project is licensed under the MIT License - see the LICENSE file for details.

Acknowledgments
GitHub API for repository data
Redis for queue management
rclone for cloud storage integration
Discord Webhooks for notifications

Support

If you encounter any issues or have questions:
Check the troubleshooting section
Search existing GitHub issues
Create a new issue with detailed information
Join our Discord server for community support

---

⚠️ Disclaimer: This tool is for educational and backup purposes. Ensure you comply with GitHub's Terms of Service and respect repository licenses when using this bot.

<p align="center">
Contributions are welcome, either request here, or email me at null@nullme.dev! Please feel free to submit a pull request.
</p>
<p align="center">
Consider donating at https://ko-fi.com/NullMeDev
</p>
<p align="center">
Made With &#x1F49C by NullMeDev.</p>

