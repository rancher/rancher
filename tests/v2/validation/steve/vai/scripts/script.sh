#!/bin/sh
set -e

MAX_RETRIES=3
BUILD_TIMEOUT=300  # 5 minutes timeout
CACHE_DIR="/var/cache/vai-query"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] $1"
}

error() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [ERROR] $1" >&2
}

# Execute command with timeout
run_with_timeout() {
    command="$1"
    timeout="$2"

    # Create a timeout process
    (
        eval "$command" &
        cmd_pid=$!

        (
            sleep $timeout
            kill $cmd_pid 2>/dev/null
        ) &
        timeout_pid=$!

        wait $cmd_pid 2>/dev/null
        kill $timeout_pid 2>/dev/null
    )
}

# Function to check if Go is installed and working
check_go() {
    if /usr/local/go/bin/go version >/dev/null 2>&1; then
        return 0
    fi
    return 1
}

# Install Go with retries
install_go() {
    mkdir -p $CACHE_DIR
    GO_ARCHIVE="$CACHE_DIR/go1.23.5.linux-amd64.tar.gz"

    for i in $(seq 1 $MAX_RETRIES); do
        log "Attempting to install Go (attempt $i of $MAX_RETRIES)..."

        if [ ! -f "$GO_ARCHIVE" ]; then
            if ! curl -L -o "$GO_ARCHIVE" https://go.dev/dl/go1.23.5.linux-amd64.tar.gz --insecure; then
                error "Failed to download Go (attempt $i)"
                continue
            fi
        fi

        if tar -C /usr/local -xzf "$GO_ARCHIVE"; then
            log "Go installed successfully"
            return 0
        else
            error "Failed to extract Go (attempt $i)"
            rm -f "$GO_ARCHIVE"
        fi
    done

    error "Failed to install Go after $MAX_RETRIES attempts"
    return 1
}

# Build vai-query with retries
build_vai_query() {
    mkdir -p /root/vai-query
    cd /root/vai-query

    # Initialize Go module if it doesn't exist
    if [ ! -f go.mod ]; then
        go mod init vai-query
    fi

    # Create or update main.go
    cat << 'EOF' > main.go
package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    "strings"
    "time"
    "context"

    "github.com/pkg/errors"
    _ "modernc.org/sqlite"
)

func main() {
    log.SetFlags(log.LstdFlags | log.Lmicroseconds)
    log.Println("Starting VAI database query...")

    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    // Clean up any existing snapshot
    os.Remove("/tmp/snapshot.db")

    // First connection to create snapshot
    log.Println("Opening connection to original database...")
    db, err := sql.Open("sqlite", "/var/lib/rancher/informer_object_cache.db")
    if err != nil {
        log.Fatalf("Failed to open original database: %v", err)
    }

    log.Println("Creating database snapshot...")
    _, err = db.ExecContext(ctx, "VACUUM INTO '/tmp/snapshot.db'")
    if err != nil {
        log.Fatalf("Failed to create snapshot: %v", err)
    }
    db.Close()

    // Wait a moment for filesystem to sync
    time.Sleep(time.Second)

    // Open the snapshot for querying
    log.Println("Opening connection to snapshot database...")
    db, err = sql.Open("sqlite", "/tmp/snapshot.db")
    if err != nil {
        log.Fatalf("Failed to open snapshot: %v", err)
    }
    defer db.Close()

    tableName := strings.ReplaceAll(os.Getenv("TABLE_NAME"), "\"", "")
    resourceName := os.Getenv("RESOURCE_NAME")

    log.Printf("Querying table '%s' for resource '%s'", tableName, resourceName)

    query := fmt.Sprintf("SELECT \"metadata.name\" FROM \"%s\" WHERE \"metadata.name\" = ?", tableName)
    stmt, err := db.PrepareContext(ctx, query)
    if err != nil {
        log.Fatalf("Failed to prepare query: %v", err)
    }
    defer stmt.Close()

    var result string
    err = stmt.QueryRowContext(ctx, resourceName).Scan(&result)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            log.Printf("Resource '%s' not found in table '%s'", resourceName, tableName)
            fmt.Printf("Resource not found\n")
        } else {
            log.Printf("Query error: %v", err)
            fmt.Printf("Query error: %v\n", err)
        }
    } else {
        log.Printf("Found resource '%s' in table '%s'", result, tableName)
        fmt.Printf("Found resource: %s\n", result)
    }

    // Clean up
    log.Println("Cleaning up snapshot...")
    os.Remove("/tmp/snapshot.db")
    log.Println("Query operation completed")
}
EOF

    for i in $(seq 1 $MAX_RETRIES); do
        log "Building vai-query (attempt $i of $MAX_RETRIES)..."

        # Get dependencies with timeout
        if ! run_with_timeout "go get github.com/pkg/errors" $BUILD_TIMEOUT; then
            error "Failed to get pkg/errors dependency (attempt $i)"
            continue
        fi

        if ! run_with_timeout "go get modernc.org/sqlite" $BUILD_TIMEOUT; then
            error "Failed to get sqlite dependency (attempt $i)"
            continue
        fi

        # Build with timeout
        if run_with_timeout "go build -o /usr/local/bin/vai-query main.go" $BUILD_TIMEOUT; then
            log "vai-query built successfully"
            return 0
        else
            error "Build failed (attempt $i)"
        fi
    done

    error "Failed to build vai-query after $MAX_RETRIES attempts"
    return 1
}

# Main execution starts here
log "Starting script execution..."

# Ensure cache directory exists
mkdir -p $CACHE_DIR

# Install Go if needed
if ! check_go; then
    log "Go not found. Installing Go..."
    if ! install_go; then
        error "Failed to install Go. Exiting."
        exit 1
    fi
fi

# Always set the PATH to include Go
export PATH=$PATH:/usr/local/go/bin

log "Checking Go version:"
go version

# Build vai-query if needed
if [ ! -f /usr/local/bin/vai-query ]; then
    log "vai-query not found. Building vai-query program..."
    if ! build_vai_query; then
        error "Failed to build vai-query. Exiting."
        exit 1
    fi
else
    log "vai-query program already exists. Using existing binary."
fi

log "Executing the query program..."
TABLE_NAME="${TABLE_NAME}" RESOURCE_NAME="${RESOURCE_NAME}" /usr/local/bin/vai-query

log "Script execution completed."