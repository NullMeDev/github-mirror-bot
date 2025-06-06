#!/bin/bash

BACKUP_DIR="${RCLONE_PATH}/repos"
GITHUB_USER="${GITHUB_USER}"
GITHUB_TOKEN="${GITHUB_TOKEN}"

if [[ -z "$BACKUP_DIR" || -z "$GITHUB_USER" || -z "$GITHUB_TOKEN" ]]; then
  echo "Error: Required environment variables RCLONE_PATH, GITHUB_USER, or GITHUB_TOKEN not set."
  exit 1
fi

mkdir -p "$BACKUP_DIR"

for repo in $(gh repo list "$GITHUB_USER" --json nameWithOwner --jq '.[].nameWithOwner'); do
  echo "Backing up $repo..."
  git
