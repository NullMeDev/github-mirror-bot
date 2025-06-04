#!/usr/bin/env bash
set -euo pipefail

# ---------- configuration ----------
source "$HOME/.config/gitbackup/env"        # exports GITHUB_TOKEN
GITHUB_USER="NullMeDev"
BACKUP_DIR="$HOME/github-mirror"
PER_PAGE=100
# ------------------------------------

mkdir -p "$BACKUP_DIR"
cd "$BACKUP_DIR"

page=1
while true; do
    api_url="https://api.github.com/user/repos?per_page=${PER_PAGE}&page=${page}"
    json=$(curl -fsSL -H "Authorization: token ${GITHUB_TOKEN}" "$api_url")

    mapfile -t urls < <(echo "$json" | jq -r '.[]?.ssh_url')
    [[ ${#urls[@]} -eq 0 ]] && break

    for url in "${urls[@]}"; do
        name=$(basename "$url" .git)
        if [[ -d "$name.git" ]]; then
            echo "Updating $name ..."
            git -C "$name.git" remote update --prune
        else
            echo "Cloning $name ..."
            git clone --mirror "$url" "$name.git"
        fi
    done
    ((page++))
done

# off-load to Google Drive and prune local
/usr/bin/rclone move "$BACKUP_DIR" "gdrive:github-mirror" --delete-after --transfers=8 --checkers=16 --fast-list

echo "Backup complete in $BACKUP_DIR"
