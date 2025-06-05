#!/usr/bin/env bash
set -euo pipefail

# ---------- configuration ----------
CONFIG_FILE="${HOME}/.config/gitbackup/config"
if [[ -f "$CONFIG_FILE" ]]; then
    source "$CONFIG_FILE"
fi

# Default values
GITHUB_TOKEN="${GITHUB_TOKEN:-}"
GITHUB_USER="${GITHUB_USER:-NullMeDev}"
BACKUP_DIR="${BACKUP_DIR:-$HOME/github-mirror}"
PER_PAGE="${PER_PAGE:-100}"
RCLONE_REMOTE="${RCLONE_REMOTE:-my2tb:github-mirror}"
LOG_FILE="${LOG_FILE:-$HOME/logs/backup.log}"
DISCORD_WEBHOOK="${DISCORD_WEBHOOK:-}"
# ------------------------------------

# Counters for Discord notification
TOTAL_REPOS=0
SUCCESS_COUNT=0
FAIL_COUNT=0
START_TIME=$(date +%s)
UPLOAD_SUCCESS=false

# Ensure log directory exists
mkdir -p "$(dirname "$LOG_FILE")"

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

# Error handling
error_exit() {
    log "ERROR: $1"
    send_discord_notification
    exit 1
}

# Discord notification function
send_discord_notification() {
    if [[ -z "$DISCORD_WEBHOOK" ]]; then
        return 0
    fi

    local end_time=$(date +%s)
    local duration=$((end_time - START_TIME))
    local duration_formatted=$(printf "%02d:%02d:%02d" $((duration/3600)) $((duration%3600/60)) $((duration%60)))

    local color="65280"  # Green
    if [[ $FAIL_COUNT -gt 0 ]] || [[ "$UPLOAD_SUCCESS" != "true" ]]; then
        if [[ $SUCCESS_COUNT -eq 0 ]]; then
            color="16711680"  # Red
        else
            color="16753920"  # Orange
        fi
    fi

    local upload_status="✅ Successfully uploaded to Google Drive"
    if [[ "$UPLOAD_SUCCESS" != "true" ]]; then
        upload_status="❌ Failed to upload to Google Drive"
    fi

    local json_payload=$(cat <<EOF
{
    "embeds": [{
        "title": "💾 Repository Backup Complete",
        "color": $color,
        "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%S.000Z)",
        "footer": {
            "text": "GitHub Backup Script"
        },
        "fields": [
            {
                "name": "📊 Total Repositories",
                "value": "$TOTAL_REPOS",
                "inline": true
            },
            {
                "name": "✅ Successfully Backed Up",
                "value": "$SUCCESS_COUNT",
                "inline": true
            },
            {
                "name": "❌ Failed",
                "value": "$FAIL_COUNT",
                "inline": true
            },
            {
                "name": "⏱️ Duration",
                "value": "$duration_formatted",
                "inline": true
            },
            {
                "name": "☁️ Cloud Storage",
                "value": "$upload_status",
                "inline": false
            }
        ]
    }]
}
EOF
)

    if ! curl -fsSL -X POST \
        -H "Content-Type: application/json" \
        -d "$json_payload" \
        "$DISCORD_WEBHOOK" >/dev/null 2>&1; then
        log "WARNING: Failed to send Discord notification"
    else
        log "Discord notification sent successfully"
    fi
}

# Check dependencies
command -v curl >/dev/null 2>&1 || error_exit "curl is required"
command -v jq >/dev/null 2>&1 || error_exit "jq is required"
command -v git >/dev/null 2>&1 || error_exit "git is required"
command -v rclone >/dev/null 2>&1 || error_exit "rclone is required"

# Check GitHub token
[[ -z "$GITHUB_TOKEN" ]] && error_exit "GITHUB_TOKEN environment variable not set"

log "Starting GitHub backup for user: $GITHUB_USER"

mkdir -p "$BACKUP_DIR"
cd "$BACKUP_DIR"

page=1

while true; do
    log "Fetching page $page..."
    
    api_url="https://api.github.com/user/repos?per_page=${PER_PAGE}&page=${page}&type=all&sort=updated"
    
    if ! json=$(curl -fsSL \
        -H "Authorization: token ${GITHUB_TOKEN}" \
        -H "Accept: application/vnd.github+json" \
        -H "User-Agent: backup-script/1.0" \
        "$api_url" 2>/dev/null); then
        error_exit "Failed to fetch repositories from GitHub API"
    fi

    # Check if we got an error response
    if echo "$json" | jq -e '.message' >/dev/null 2>&1; then
        error_msg=$(echo "$json" | jq -r '.message')
        error_exit "GitHub API error: $error_msg"
    fi

    mapfile -t urls < <(echo "$json" | jq -r '.[]?.ssh_url // empty')
    mapfile -t names < <(echo "$json" | jq -r '.[]?.name // empty')
    
    [[ ${#urls[@]} -eq 0 ]] && break

    for i in "${!urls[@]}"; do
        url="${urls[$i]}"
        name="${names[$i]}"
        
        [[ -z "$url" || -z "$name" ]] && continue
        
        ((TOTAL_REPOS++))
        
        if [[ -d "$name.git" ]]; then
            log "Updating $name..."
            if git -C "$name.git" remote update --prune 2>>"$LOG_FILE"; then
                ((SUCCESS_COUNT++))
            else
                log "WARNING: Failed to update $name"
                ((FAIL_COUNT++))
                continue
            fi
        else
            log "Cloning $name..."
            if git clone --mirror "$url" "$name.git" 2>>"$LOG_FILE"; then
                ((SUCCESS_COUNT++))
            else
                log "WARNING: Failed to clone $name"
                ((FAIL_COUNT++))
                continue
            fi
        fi
    done
    
    # Break if we got fewer repos than requested (last page)
    [[ ${#urls[@]} -lt $PER_PAGE ]] && break
    
    ((page++))
done

log "Processed $TOTAL_REPOS repositories (Success: $SUCCESS_COUNT, Failed: $FAIL_COUNT)"

# Upload to remote storage
if [[ -n "$RCLONE_REMOTE" ]]; then
    log "Uploading to remote storage: $RCLONE_REMOTE"
    if rclone sync "$BACKUP_DIR" "$RCLONE_REMOTE" \
        --transfers=8 \
        --checkers=16 \
        --fast-list \
        --progress \
        --log-file="$LOG_FILE" \
        --log-level=INFO; then
        log "Successfully synced to remote storage"
        UPLOAD_SUCCESS=true
    else
        log "WARNING: Failed to sync to remote storage"
        UPLOAD_SUCCESS=false
    fi
fi

# Send Discord notification
send_discord_notification

log "Backup completed successfully"
