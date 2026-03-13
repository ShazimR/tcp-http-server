#!/usr/bin/env bash
set -Eeuo pipefail

REPO_DIR="${HOME}/tcp-http-server"
REMOTE="origin"
BRANCH="main"
LOCK_FILE="/tmp/tcp-http-server-deploy.lock"
APP_URL="http://127.0.0.1:8080/"
TARGET_SHA="${1:-}"

log() {
    printf '[%s] %s\n' "$(date -u +"%Y-%m-%d %H:%M:%S")" "$*"
}

rollback() {
    trap - ERR
    local exit_code=$?
    if [[ -n "${PREV_SHA:-}" ]]; then
        log "Deploy failed. Rolling back to ${PREV_SHA}..."
        cd "$REPO_DIR"
        git reset --hard "$PREV_SHA"
        docker compose up --build -d
        log "Rollback complete."
    fi
    exit "$exit_code"
}

exec 9>"$LOCK_FILE"
if ! flock -n 9; then
    log "Another deploy is already running. Exiting."
    exit 1
fi

trap rollback ERR

cd "$REPO_DIR"

log "Starting deploy."
git fetch --prune "$REMOTE"

PREV_SHA="$(git rev-parse HEAD)"
log "Previous commit: ${PREV_SHA}."

if [[ -n  "$TARGET_SHA" ]]; then
    log "Deploying exact commit: ${TARGET_SHA}."
    git cat-file -e "${TARGET_SHA}^{commit}"
    git reset --hard "$TARGET_SHA"
else
    log "Deploying latest ${REMOTE}/${BRANCH}."
    git reset --hard "${REMOTE}/${BRANCH}"
fi

NEW_SHA="$(git rev-parse HEAD)"
log "New commit: ${NEW_SHA}."

log "Building and starting containers..."
docker compose up --build -d

log "Waiting for app startup..."
sleep 3

log "Running health check against ${APP_URL}."
curl --fail --silent --show-error --max-time 10 "$APP_URL" >/dev/null

log "Cleaning up dangling images..."
docker image prune -f >/dev/null || true

log "Deploy succeeded: ${NEW_SHA}"
