# Upgrade

For the Fleet upgrade tests, these tests are designed to work with the remote GitHub repository fleet-examples, It is need to have a cluster configuration of 3 workers

## Follow these steps before running the upgrade tests:

1) Clone the [rancher/fleet-examples](https://github.com/rancher/fleet-examples) repository.
2) Create a [personal token](https://github.com/settings/tokens)
3) Generate an HTTP Basic Auth Secret using:
  > 1) Your GitHub username as the username
  > 2) The access token as the password
  > 3) Set the secret name as "gitsecret"
  > 4) Set Namespace to the "fleet-default" namespace