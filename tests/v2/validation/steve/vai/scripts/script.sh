#!/bin/sh
set -e

echo "Starting script execution..."

# Function to check if Go is installed and working
check_go() {
    if /usr/local/go/bin/go version >/dev/null 2>&1; then
        return 0
    fi
    return 1
}

# Install Go if not already installed
if ! check_go; then
    echo "Go not found. Installing Go..."
    curl -L -o go1.22.4.linux-amd64.tar.gz https://go.dev/dl/go1.22.4.linux-amd64.tar.gz --insecure
    tar -C /usr/local -xzf go1.22.4.linux-amd64.tar.gz
    rm go1.22.4.linux-amd64.tar.gz
    echo "Go installed successfully."
else
    echo "Go is already installed."
fi

# Always set the PATH to include Go
export PATH=$PATH:/usr/local/go/bin

echo "Checking Go version:"
go version

# Check if vai-query already exists
if [ ! -f /usr/local/bin/vai-query ]; then
    echo "vai-query not found. Building vai-query program..."
    mkdir -p /root/vai-query
    cd /root/vai-query

    # Initialize Go module if it doesn't exist
    if [ ! -f go.mod ]; then
        go mod init vai-query
    fi

    # Create or update main.go
    cat << EOF > main.go
package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    "strings"

    "github.com/pkg/errors"
    _ "modernc.org/sqlite"
)

func main() {
    db, err := sql.Open("sqlite", "/var/lib/rancher/informer_object_cache.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    fmt.Println("Creating database snapshot...")
    _, err = db.Exec("VACUUM INTO '/tmp/snapshot.db'")
    if err != nil {
        log.Fatal(err)
    }

    tableName := strings.ReplaceAll(os.Getenv("TABLE_NAME"), "\"", "")
    resourceName := os.Getenv("RESOURCE_NAME")

    query := fmt.Sprintf("SELECT \"metadata.name\" FROM \"%s\" WHERE \"metadata.name\" = ?", tableName)
    stmt, err := db.Prepare(query)
    if err != nil {
        log.Fatal(err)
    }
    defer stmt.Close()

    var result string
    err = stmt.QueryRow(resourceName).Scan(&result)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            fmt.Println("Resource not found")
        } else {
            log.Fatal(err)
        }
    } else {
        fmt.Println("Found resource:", result)
    }
}
EOF

    # Get dependencies
    go get github.com/pkg/errors
    go get modernc.org/sqlite

    # Build the program
    go build -o /usr/local/bin/vai-query main.go
    echo "Pure Go vai-query program built successfully."
else
    echo "vai-query program already exists. Using existing binary."
fi

echo "Executing the query program..."
TABLE_NAME="${TABLE_NAME}" RESOURCE_NAME="${RESOURCE_NAME}" /usr/local/bin/vai-query

echo "Script execution completed."