Shell Configs
Getting Started
Your GO test_package should be set to shell. Your GO suite should be set to -run ^TestShellTestSuite$. 

In your config file, set the following:

rancher:
  host: "rancher_server_address" 
  adminToken: "rancher_admin_token"
  insecure: true/optional
  cleanup: true/optional
  shellImage : "rancher/shell:<Version>"

While validating the shell P0 checks, set the shellImage version to the latest version. We validate the shell latest image that is available to that of the image we have on our rancher server.