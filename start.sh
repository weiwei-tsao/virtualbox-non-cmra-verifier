#!/bin/bash

# ==============================================================================
# 🚀 Starts both frontend and backend services for Virtual Box Verifier
# ==============================================================================

# Stop execution if any command fails
set -e

# Navigate to the project root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
cd "$SCRIPT_DIR"

echo "========================================================"
echo "🚀 Starting Virtual Box Verifier local development..."
echo "========================================================"

# 1. Check if backend .env.local is present
API_ENV_FILE="apps/api/.env.local"
if [ ! -f "$API_ENV_FILE" ]; then
    echo "⚠️  Warning: Backend environment file ($API_ENV_FILE) not found!"
    echo "🛠️  Creating a default one with safe mock configurations..."
    
    cat > "$API_ENV_FILE" <<'EOF'
PORT=8080
GIN_MODE=debug
ALLOWED_ORIGINS=http://localhost:5173

FIREBASE_PROJECT_ID=your-project-id
FIREBASE_CREDS_FILE=service-account.json

# MOCK mode on by default. Turn to 'false' if using real smarty keys.
SMARTY_AUTH_ID=your-smarty-id
SMARTY_AUTH_TOKEN=your-smarty-token
SMARTY_MOCK=true

CRAWLER_CONCURRENCY=5
EOF
    
    echo "✅ Created default $API_ENV_FILE (SMARTY_MOCK=true)"
else
    echo "✅ Backend environment ($API_ENV_FILE) found."
fi

# 2. Check for node_modules and install if missing
if [ ! -d "node_modules" ]; then
    echo "📦 node_modules not found. Running pnpm install..."
    pnpm install
else
    echo "✅ Dependencies found."
fi

# 3. Start services via pnpm workspace (concurrently)
echo "🔥 Starting frontend (:5173) and API (:8080) concurrently..."
echo "👉 Press Ctrl+C to stop all services."
echo ""

# Start backend in the background
echo "🚀 [API] Starting backend..."
cd apps/api
go run ./cmd/server &
BACKEND_PID=$!
cd ../..

# Start frontend in the background
echo "🚀 [WEB] Starting frontend..."
pnpm dev:web &
FRONTEND_PID=$!

# Trap Ctrl+C (SIGINT) to kill both background processes
trap "echo -e '\n🛑 Stopping services...'; kill $BACKEND_PID $FRONTEND_PID 2>/dev/null; exit 0" SIGINT SIGTERM

# Wait for both processes
wait $BACKEND_PID $FRONTEND_PID
