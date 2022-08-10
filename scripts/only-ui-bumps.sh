#!/bin/bash
echo "Checking if we can skip CI because of an only UI/dashboard change"
echo "Environment variable DRONE_BUILD_EVENT is ${DRONE_BUILD_EVENT}"
# Only run check if Drone build event is 'push' or 'pull_request'
if [ "${DRONE_BUILD_EVENT}" = "push" ] || [ "${DRONE_BUILD_EVENT}" = "pull_request" ]; then
    # Check if there is only one changed file and if its 'package/Dockerfile'
    if [ $(git diff HEAD~1 --name-only | wc -l) -eq 1 ] && [ $(git diff HEAD~1 --name-only) = "package/Dockerfile" ]; then
        echo "Only package/Dockerfile changes found"
        # Check if only CATTLE_UI_VERSION and CATTLE_DASHBOARD_UI_VERSION are changed in 'package/Dockerfile'
        if [ -z "$(git diff -U0 HEAD~1 -- package/Dockerfile | tail -n +5 | grep -v ^@@ | egrep -v "CATTLE_UI_VERSION|CATTLE_DASHBOARD_UI_VERSION")" ]; then
            echo "Skipping CI because it is only UI/dashboard change"
            exit 0
        fi
    fi
fi
echo "Not skipping CI, not an only UI/dashboard change"
exit 1
