#!/bin/sh
set -ue

FINAL_RC_PARTIAL_COMMIT_MESSAGE=$(yq eval '.FINAL_RC_PARTIAL_COMMIT_MESSAGE' build.yml)
FINAL_RC_PRERELEASE_TITLE=$(yq eval '.FINAL_RC_PRERELEASE_TITLE' build.yml)
FINAL_RC_PRERELEASE_REGEX=$(yq eval '.FINAL_RC_PRERELEASE_REGEX' build.yml)
FINAL_RC_FILES=$(yq eval '.FINAL_RC_FILES[]' build.yml)
HEAD_COMMIT_MESSAGE=$(git log -2 --pretty=format:"%s")
RELEASE_TITLE=$(echo "$RELEASE_TITLE")
BAD_FILES=false

if echo "$HEAD_COMMIT_MESSAGE" | grep -q "$FINAL_RC_PARTIAL_COMMIT_MESSAGE" || echo "$RELEASE_TITLE" | grep -Eq "^${FINAL_RC_PRERELEASE_TITLE}${FINAL_RC_PRERELEASE_REGEX}"; then
    echo "Starting check..."
    for FILE in $FINAL_RC_FILES; do
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
        echo "Check failed, some files don't match the expected dependencies for a final release candidate"
        exit 1
    fi

    echo "Check completed successfully"
    exit 0
fi

echo "Skipped check"
exit 0
