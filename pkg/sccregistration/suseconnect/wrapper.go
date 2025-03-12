package suseconnect

import "github.com/SUSE/connect-ng/pkg/connection"

func DefaultConnectionOptions() connection.Options {
	// TODO: I believe this is creating the options for the API "app"
	// So this doesn't necessarily mean these have to match Rancher on the cluster.
	// Rather the details about the HTTP client talking to SCC
	return connection.DefaultOptions("rancher-scc-integration", "v0.0.1", "todo lang")
}

func DefaultRancherConnection() *connection.ApiConnection {
	options := DefaultConnectionOptions()
	return connection.New(options, connection.NoCredentials{})
}
