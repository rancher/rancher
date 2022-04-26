#!/bin/bash
echo "Checking if we can skip CI"
# Can only compare diff if branch is set
if [ -n "${DRONE_BRANCH}" ]; then
    echo "Environment variable DRONE_BRANCH is ${DRONE_BRANCH}"
    # Check if there is only one changed file and if its 'package/Dockerfile'
    if [ $(git diff ${DRONE_BRANCH} --name-only | wc -l) -eq 1 ] && [ $(git diff ${DRONE_BRANCH} --name-only) = "package/Dockerfile" ]; then
        echo "Only package/Dockerfile changes found"
        # Check if only CATTLE_UI_VERSION and CATTLE_DASHBOARD_UI_VERSION are changed in 'package/Dockerfile'
        if [ -z "$(git diff -U0 ${DRONE_BRANCH} package/Dockerfile | tail -n +5 | grep -v ^@@ | egrep -v "CATTLE_UI_VERSION|CATTLE_DASHBOARD_UI_VERSION")" ]; then
            echo "Skipping CI because it is only UI/dashboard change"
            exit 0
        fi
    fi
fi
echo "Not all conditions met to skip CI"
exit 1
