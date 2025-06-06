#!/bin/bash

BACKUP_DIR="${RCLONE_PATH}/repos"
GITHUB_USER="${GITHUB_USER}"
GITHUB_TOKEN="${GITHUB_TOKEN}"

if [[ -z "$BACKUP_DIR" || -z "$GITHUB_USER" || -z "$GITHUB_TOKEN" ]]; then
  echo "Error: Required environment variables RCLONE_PATH, GITHUB_USER, or GITHUB_TOKEN not set."
  exit 1
fi

mkdir -p "$BACKUP_DIR"

# List repos via GitHub CLI (requires gh installed and authenticated)
repos=$(gh repo list "$GITHUB_USER" --json nameWithOwner --jq '.[].nameWithOwner')

for repo in $repos; do
  echo "Backing up $repo..."
  repo_dir="$BACKUP_DIR/$(echo $repo | tr '/' '_')"
  
  if [[ -d "$repo_dir" ]]; then
    echo "Repository already cloned at $repo_dir, pulling latest changes"
    git -C "$repo_dir" pull
  else
    git clone --depth=1 "https://github.com/$repo.git" "$repo_dir"
  fi

  if [[ $? -ne 0 ]]; then
    echo "Failed to backup $repo"
  else
    echo "Successfully backed up $repo"
  fi
done
