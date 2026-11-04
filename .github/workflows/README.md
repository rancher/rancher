# Description of GitHub Actions in this repository

## Go Get (`go-get.yml`)

Go Get can be used to automate updating Go modules in this repository. It will run `make go-get` which is a helper script for running `go get -d $GOGET_MODULE@$GOGET_VERSION` in all needed places, commit and create a pull request.

If `Username of the source for this workflow run` is set, the username will be mentioned in the pull request and configured as assignee. This was added for automated workflows, where the user and URL can be used to link back to the source of the trigger.

If `URL of the source for this workflow run` is set, the URL will be mentioned in the pull request. This was added for automated workflows, where the user and URL can be used to link back to the source of the trigger.

## Create doc issue (`create-doc-issue.yml`)

Organization members can comment `/create-doc-issue` on an issue (not a pull request) to create a documentation issue in the same organization's `rancher-product-docs` repository, or `prime-docs` if the original issue has the `prime` label. The new issue's title is prefixed with `document:`, and its body contains the original issue's link followed by `More information to be added upon request.`

The workflow uses `CREATE_DOCS_PAT`, which needs permission to read organization membership and create issues in both documentation repositories and comment on the original issue. To detect private members, the PAT must belong to an organization member and have the `read:org` scope. If the organization uses SAML SSO, authorize the PAT for the organization. The membership check returns `404` both when the actor is not a member and when the token cannot see a private member, so these requirements are necessary for private members to be detected.

The workflow copies the milestone by matching its title in the target repository and comments on the original issue with the new issue's link. If the milestone is missing in the target repository, it creates the issue without one and notes this in the confirmation comment.
