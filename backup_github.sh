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
# ------------------------------------

# Ensure log directory exists
mkdir -p "$(dirname "$LOG_FILE")"

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

# Error handling
error_exit() {
    log "ERROR: $1"
    exit 1
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
total_repos=0

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
        
        if [[ -d "$name.git" ]]; then
            log "Updating $name..."
            if ! git -C "$name.git" remote update --prune 2>>"$LOG_FILE"; then
                log "WARNING: Failed to update $name"
                continue
            fi
        else
            log "Cloning $name..."
            if ! git clone --mirror "$url" "$name.git" 2>>"$LOG_FILE"; then
                log "WARNING: Failed to clone $name"
                continue
            fi
        fi
        
        ((total_repos++))
    done
    
    # Break if we got fewer repos than requested (last page)
    [[ ${#urls[@]} -lt $PER_PAGE ]] && break
    
    ((page++))
done

log "Processed $total_repos repositories"

# Upload to remote storage
if [[ -n "$RCLONE_REMOTE" ]]; then
    log "Uploading to remote storage: $RCLONE_REMOTE"
    if ! rclone sync "$BACKUP_DIR" "$RCLONE_REMOTE" \
        --transfers=8 \
        --checkers=16 \
        --fast-list \
        --progress \
        --log-file="$LOG_FILE" \
        --log-level=INFO; then
        log "WARNING: Failed to sync to remote storage"
    else
        log "Successfully synced to remote storage"
    fi
fi

log "Backup completed successfully"
