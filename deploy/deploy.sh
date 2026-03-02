#!/bin/bash
# Decent Notes — build & deploy via Docker Compose
# Called by post-receive hook or run manually from the project root.

set -euo pipefail

DEPLOY_DIR="$(cd "$(dirname "$0")/.." && pwd)"
LOG_FILE="$DEPLOY_DIR/decent-notes-info.log"
ENV_FILE="$DEPLOY_DIR/.env"
TIMESTAMP="$(date '+%Y-%m-%d %H:%M:%S')"

log() {
    echo "[$TIMESTAMP] $1" | tee -a "$LOG_FILE"
}

# ── Load config ───────────────────────────────────────────
if [ ! -f "$ENV_FILE" ]; then
    log "No .env file found — creating from .env.example with defaults."
    cp "$DEPLOY_DIR/.env.example" "$ENV_FILE"
fi

source "$ENV_FILE"
PORT="${PORT:-5050}"

# ── Stop existing container before checking port ──────────
if docker compose ps --quiet 2>/dev/null | grep -q .; then
    log "Stopping existing container..."
    docker compose down 2>&1 | tee -a "$LOG_FILE"
fi

# ── Check host port availability ──────────────────────────
check_port() {
    local port=$1
    if ss -tlnp 2>/dev/null | grep -q ":${port} "; then
        return 1  # port in use
    fi
    return 0  # port free
}

suggest_port() {
    local start=$1
    for p in $(seq "$start" $((start + 100))); do
        if check_port "$p"; then
            echo "$p"
            return
        fi
    done
    echo ""
}

if ! check_port "$PORT"; then
    SUGGESTED=$(suggest_port $((PORT + 1)))
    log "ERROR: Host port $PORT is already in use."
    if [ -n "$SUGGESTED" ]; then
        log "  Suggested available port: $SUGGESTED"
        log "  Update PORT=$SUGGESTED in $ENV_FILE and re-deploy."
    else
        log "  Could not find a free port nearby. Check your system."
    fi
    log "Deploy aborted."
    exit 1
fi

log "Port $PORT is available."

# ── Build and deploy ──────────────────────────────────────
cd "$DEPLOY_DIR"

log "Building Docker image..."
docker compose build --no-cache 2>&1 | tee -a "$LOG_FILE"

log "Starting container on port $PORT..."
docker compose up -d 2>&1 | tee -a "$LOG_FILE"

# ── Verify ────────────────────────────────────────────────
sleep 2
if curl -sf "http://localhost:${PORT}/health" > /dev/null 2>&1; then
    log "SUCCESS: Decent Notes is running on http://localhost:${PORT}"
else
    log "WARNING: Container started but health check failed. Check 'docker compose logs'."
fi
