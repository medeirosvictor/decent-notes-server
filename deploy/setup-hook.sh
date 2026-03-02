#!/bin/bash
# One-time setup: install the post-receive hook on the homeserver.
#
# Run from your local machine (Windows/WSL) where S:/ is mapped:
#   bash deploy/setup-hook.sh
#
# Or run on the server directly with the bare repo path:
#   bash deploy/setup-hook.sh /path/to/decent-notes.git

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BARE_REPO="${1:-S:/projects/decent-notes.git}"
DEPLOY_DIR="/opt/decent-notes"

echo "Installing post-receive hook..."
echo "  Bare repo: $BARE_REPO"
echo "  Deploy dir: $DEPLOY_DIR"
echo ""

# Copy hook
cp "$SCRIPT_DIR/post-receive" "$BARE_REPO/hooks/post-receive"
chmod +x "$BARE_REPO/hooks/post-receive"

echo "Hook installed at: $BARE_REPO/hooks/post-receive"
echo ""
echo "── Next steps (on the server) ──────────────────────"
echo "  1. Ensure the deploy directory exists and is writable:"
echo "       sudo mkdir -p $DEPLOY_DIR"
echo "       sudo chown \$(whoami):\$(whoami) $DEPLOY_DIR"
echo ""
echo "  2. Ensure Docker is installed and your user can run it:"
echo "       docker --version"
echo "       docker compose version"
echo ""
echo "  3. Push to the homeserver remote to trigger a deploy:"
echo "       git push homeserver master"
echo ""
echo "  4. (Optional) Create $DEPLOY_DIR/.env to customize the port:"
echo "       PORT=5050"
echo ""
