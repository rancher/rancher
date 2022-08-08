#!/bin/bash
echo "Checking if we can skip CI"
echo "Environment variable DRONE_BUILD_EVENT is ${DRONE_BUILD_EVENT}"
# Only run check if Drone build event is 'push' or 'pull_request'
if [ "${DRONE_BUILD_EVENT}" = "push" ] || [ "${DRONE_BUILD_EVENT}" = "pull_request" ]; then
    # In case of pull_ request, we can diff from latest commit (as Drone merges the changes as one commit)
    if [ "${DRONE_BUILD_EVENT}" = "pull_request" ]; then
        # Check if there is only one changed file and if its 'package/Dockerfile'
        if [ $(git diff HEAD~1 --name-only | wc -l) -eq 1 ] && [ $(git diff HEAD~1 --name-only) = "package/Dockerfile" ]; then
            echo "Only package/Dockerfile changes found"
            # Check if only CATTLE_UI_VERSION and CATTLE_DASHBOARD_UI_VERSION are changed in 'package/Dockerfile'
            if [ -z "$(git diff -U0 HEAD~1 package/Dockerfile | tail -n +5 | grep -v ^@@ | egrep -v "CATTLE_UI_VERSION|CATTLE_DASHBOARD_UI_VERSION")" ]; then
                echo "Skipping CI because it is only UI/dashboard change"
                exit 0
            fi
        fi
    # In case of push, we need to get the diff from Drone's environment variables
    elif [ "${DRONE_BUILD_EVENT}" = "push" ]; then
        # Check for required environment variables
        if [ -z "${DRONE_COMMIT_BEFORE}" ] || [ -z "${DRONE_COMMIT_AFTER}" ]; then
            echo "Required environment variables not present; DRONE_COMMIT_BEFORE: ${DRONE_COMMIT_BEFORE}, DRONE_COMMIT_AFTER: ${DRONE_COMMIT_AFTER}"
            exit 1
        fi
        # Check if there is only one changed file and if its 'package/Dockerfile'
        if [ $(git diff $DRONE_COMMIT_BEFORE $DRONE_COMMIT_AFTER --name-only | wc -l) -eq 1 ] && [ $(git diff $DRONE_COMMIT_BEFORE $DRONE_COMMIT_AFTER --name-only) = "package/Dockerfile" ]; then
            echo "Only package/Dockerfile changes found"
            # Check if only CATTLE_UI_VERSION and CATTLE_DASHBOARD_UI_VERSION are changed in 'package/Dockerfile'
            if [ -z "$(git diff -U0 $DRONE_COMMIT_BEFORE $DRONE_COMMIT_AFTER package/Dockerfile | tail -n +5 | grep -v ^@@ | egrep -v "CATTLE_UI_VERSION|CATTLE_DASHBOARD_UI_VERSION")" ]; then
                echo "Skipping CI because it is only UI/dashboard change"
                exit 0
            fi
        fi
    fi
fi
echo "Not skipping CI, not an only UI/dashboard change"
exit 1
