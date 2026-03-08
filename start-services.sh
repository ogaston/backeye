#!/usr/bin/env bash
# Start both backeye services (tracker + coordinator).
# Usage: ./start-services.sh [location]
#   location: optional, e.g. "front-door" or "parking-lot" (default: unknown)
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COORDINATOR_DIR="${SCRIPT_DIR}/coordinator"
TRACKER_DIR="${SCRIPT_DIR}/tracker"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# -----------------------------------------------------------------------------
# Tool validation
# -----------------------------------------------------------------------------
check_command() {
    if command -v "$1" &>/dev/null; then
        log_info "$1 found: $(command -v "$1") ($($1 --version 2>&1 | head -1 || true))"
        return 0
    else
        log_error "$1 is not installed or not in PATH"
        return 1
    fi
}

check_go() {
    if ! check_command go; then
        log_error "Install Go from https://go.dev/dl/"
        return 1
    fi
    # Verify go version is reasonable (1.18+)
    local version
    version=$(go version 2>&1 | grep -oE 'go[0-9]+\.[0-9]+' | head -1)
    log_info "Go version: $version"
    return 0
}

check_python() {
    if ! check_command python3; then
        log_error "Install Python 3 from https://www.python.org/downloads/"
        return 1
    fi
    return 0
}

check_pip() {
    if python3 -m pip --version &>/dev/null; then
        log_info "pip found: $(python3 -m pip --version 2>&1)"
        return 0
    else
        log_error "pip is not available. Run: python3 -m ensurepip --upgrade"
        return 1
    fi
}

check_python_packages() {
    local missing=()
    # ultralytics -> import ultralytics; opencv-python -> import cv2
    python3 -c "import ultralytics" 2>/dev/null || missing+=("ultralytics")
    python3 -c "import cv2" 2>/dev/null || missing+=("opencv-python")
    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Missing Python packages: ${missing[*]}"
        log_info "Install with: python3 -m pip install ultralytics opencv-python"
        return 1
    fi
    log_info "Python packages OK (ultralytics, opencv-python)"
    return 0
}

validate_tools() {
    log_info "Validating required tools..."
    local failed=0
    check_go || ((failed++))
    check_python || ((failed++))
    check_pip || ((failed++))
    check_python_packages || ((failed++))
    if [[ $failed -gt 0 ]]; then
        log_error "Validation failed. Install missing tools and try again."
        exit 1
    fi
    log_info "All tools validated successfully."
}

# -----------------------------------------------------------------------------
# Service management
# -----------------------------------------------------------------------------
TRACKER_PID=""
COORDINATOR_PID=""

cleanup() {
    log_info "Shutting down services..."
    if [[ -n "$COORDINATOR_PID" ]] && kill -0 "$COORDINATOR_PID" 2>/dev/null; then
        kill "$COORDINATOR_PID" 2>/dev/null || true
        wait "$COORDINATOR_PID" 2>/dev/null || true
        log_info "Coordinator stopped."
    fi
    if [[ -n "$TRACKER_PID" ]] && kill -0 "$TRACKER_PID" 2>/dev/null; then
        kill "$TRACKER_PID" 2>/dev/null || true
        wait "$TRACKER_PID" 2>/dev/null || true
        log_info "Tracker stopped."
    fi
    exit 0
}

trap cleanup SIGINT SIGTERM EXIT

# -----------------------------------------------------------------------------
# Main
# -----------------------------------------------------------------------------
main() {
    local location="${1:-unknown}"
    validate_tools

    log_info "Starting tracker (must start first - coordinator connects to it)..."
    cd "$TRACKER_DIR"
    python3 tracker-service.py &
    TRACKER_PID=$!
    cd "$SCRIPT_DIR"

    # Give tracker time to bind to port 8089
    sleep 2
    if ! kill -0 "$TRACKER_PID" 2>/dev/null; then
        log_error "Tracker failed to start."
        exit 1
    fi
    log_info "Tracker started (PID: $TRACKER_PID)"

    log_info "Building and starting coordinator (location: $location)..."
    cd "$COORDINATOR_DIR"
    go build -o backeye . || { log_error "Coordinator build failed."; exit 1; }
    ./backeye -location "$location" &
    COORDINATOR_PID=$!
    cd "$SCRIPT_DIR"
    log_info "Coordinator started (PID: $COORDINATOR_PID)"

    log_info "Both services running. Press Ctrl+C to stop."
    wait
}

main "$@"
