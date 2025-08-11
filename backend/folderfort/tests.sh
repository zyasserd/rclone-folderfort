#!/usr/bin/env bash

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RCLONE_DIR="$SCRIPT_DIR/../.."
DIAG_DIR="$SCRIPT_DIR/diagnostics"

# Get git commit hash (short version)
cd "$RCLONE_DIR"
GIT_HASH=$(git rev-parse --short HEAD)
echo "Current git hash: $GIT_HASH"

# Create directory with git hash name (no error if exists)
mkdir -p "$DIAG_DIR/$GIT_HASH"
echo "Created/using directory: diagnostics/$GIT_HASH"

echo "Running fs_unit tests..."         # ~ 15 mins
cd "$RCLONE_DIR/backend/folderfort"
go test -v -remote ff_test: -timeout 0 > "$DIAG_DIR/$GIT_HASH/fs_unit.txt" 2>&1
# -skip <test name>
# -run <test name>
# -verbose

echo "Running fs_operations tests..."   # ~ 25 mins  
cd "$RCLONE_DIR/fs/operations"
go test -v -remote ff_test: -timeout 0 > "$DIAG_DIR/$GIT_HASH/fs_op.txt" 2>&1

echo "Running fs_sync tests..."         # ~ 60 mins
cd "$RCLONE_DIR/fs/sync"
go test -v -remote ff_test: -timeout 0 > "$DIAG_DIR/$GIT_HASH/fs_sync.txt" 2>&1

echo "All tests completed. Results saved in diagnostics/$GIT_HASH/"
