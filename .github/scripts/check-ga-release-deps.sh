#!/bin/sh
set -ue

FILE_PATHS="
    Dockerfile.dapper 
    go.mod 
    ./package/Dockerfile 
    ./pkg/apis/go.mod 
    ./pkg/settings/setting.go 
    ./scripts/package-env
"

RELEASE_TITLE=$(echo "$RELEASE_TITLE" | tr '[:upper:]' '[:lower:]')
# COUNT_FILES=$(echo "$FILE_PATHS" | grep -c "$")
BAD_FILES=false

# echo "Starting check, $COUNT_FILES files detected..."

if echo "$RELEASE_TITLE" | grep -Eq '^(release v([0-9]{1,2}|100)\.[0-9]{1,100}\.[0-9]{1,100}|v([0-9]{1,2}|100)\.[0-9]{1,100}\.[0-9]{1,100})$'; then
    for FILE in $FILE_PATHS; do
        if grep -q -E '\-rc[0-9]+' "$FILE"; then
            BAD_FILES=true
            echo "error: ${FILE} contains rc tags."
        fi

        if grep -q -Eo 'dev-v[0-9]+\.[0-9]+' "$FILE"; then
            BAD_FILES=true
            echo "error: ${FILE} contains dev dependencies."
        fi
    done

    if [ "${BAD_FILES}" = true ]; then
        echo "Check failed, some files don't match the expected dependencies for a GA release"
        exit 1
    fi

    echo "Check completed successfully"
    exit 0
fi

echo "Skipped check"
exit 0
