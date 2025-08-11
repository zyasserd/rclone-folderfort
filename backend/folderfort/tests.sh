#!/usr/bin/env bash

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RCLONE_DIR="$SCRIPT_DIR/../.."
DIAG_DIR="$SCRIPT_DIR/diagnostics"


# Print help message
show_help() {
    echo "Usage: $0 <remote_name>"
    echo "Runs the test suite for the specified remote."
    echo
    echo "Arguments:"
    echo "  <remote_name>   The rclone remote to use (e.g. ff_test:). Required."
    echo
    echo "Examples:"
    echo "  $0 folderfort_test:"
    echo "  $0 myremote:"
}

# Show help if requested or if no argument is given
if [[ "$1" == "-h" || "$1" == "--help" || -z "$1" ]]; then
    show_help
    exit 1
fi

REMOTE_NAME="$1"

# Get git commit hash (short version)
cd "$RCLONE_DIR"
GIT_HASH=$(git rev-parse --short HEAD)
echo "Current git hash: $GIT_HASH"

# Create directory with git hash name (no error if exists)
mkdir -p "$DIAG_DIR/$GIT_HASH"
echo "Created/using directory: diagnostics/$GIT_HASH"

echo "Running fs_unit tests..."         # ~ 15 mins
cd "$RCLONE_DIR/backend/folderfort"
go test -v -remote "$REMOTE_NAME" -timeout 0 > "$DIAG_DIR/$GIT_HASH/fs_unit.txt" 2>&1
# -skip <test name>
# -run <test name>
# -verbose

echo "Running fs_operations tests..."   # ~ 25 mins  
cd "$RCLONE_DIR/fs/operations"
go test -v -remote "$REMOTE_NAME" -timeout 0 > "$DIAG_DIR/$GIT_HASH/fs_op.txt" 2>&1

echo "Running fs_sync tests..."         # ~ 60 mins
cd "$RCLONE_DIR/fs/sync"
go test -v -remote "$REMOTE_NAME" -timeout 0 > "$DIAG_DIR/$GIT_HASH/fs_sync.txt" 2>&1

echo "All tests completed. Results saved in diagnostics/$GIT_HASH/"
