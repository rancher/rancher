# Description of GitHub Actions in this repository

## Go Get (`go-get.yml`)

Go Get can be used to automate updating Go modules in this repository. It will run `make go-get` which is a helper script for running `go get -d $GOGET_MODULE@$GOGET_VERSION` in all needed places, commit and create a pull request.

If `Username of the source for this workflow run` is set, the username will be mentioned in the pull request and configured as assignee. This was added for automated workflows, where the user and URL can be used to link back to the source of the trigger.

If `URL of the source for this workflow run` is set, the URL will be mentioned in the pull request. This was added for automated workflows, where the user and URL can be used to link back to the source of the trigger.
