Reverse Tunneling Dialer
========================

remotedialer creates a two-way connection between a server and a client, so
that a `net.Dial` can be performed on the server and actually connects to the
client running services accessible on localhost or behind a NAT or firewall.

Architecture
------------

### Abstractions

remotedialer consists of structs that organize and abstract a TCP connection
between a server and a client. Both client and server use ``Session``s, which
provides a means to make a network connection to a remote endpoint and keep
track of incoming connections. A server uses the ``Server`` object which
contains a ``sessionManager`` which governs one or more ``Session``s, while a
client creates a ``Session`` directly. A ``connection`` implements the
``io.Reader`` and ``io.WriteCloser`` interfaces, so it can be read from and
written to directly. The ``connection``'s internal ``readBuffer`` monitors the
size of the data it is carrying and uses ``backPressure`` to pause incoming
data transfer until the amount of data is below a threshold.

![](./docs/remotedialer.png)

### Data flow

![](./docs/remotedialer-flow.png)

A client establishes a session with a server using the server's URL. The server
upgrades the connection to a websocket connection which it uses to create a
``Session``. The client then also creates a ``Session`` with the websocket
connection.

The client sits in front of some kind of HTTP server, most often a kubernetes
API server, and acts as a reverse proxy for that HTTP resource. When a user
requests a resource from this remote resource, request first goes to the
remotedialer server. The application containing the server is responsible for
routing the request to the right client connection.

The request is sent through the websocket connection to the client and read
into the client's connection buffer. A pipe is created between the client and
the HTTP service which continually copies data between each socket. The request
is forwarded through the pipe to the remote service, draining the buffer. The
service's response is copied back to the client's buffer, and then forwarded
back to the server and copied into the server connection's own buffer. As the
user reads the response, the server's buffer is drained.

The pause/resume mechanism checks the size of the buffer for both the client
and server. If it is greater than the threshold, a ``PAUSE`` message is sent
back to the remote connection, as a suggestion not to send any more data. As
the buffer is drained, either by the pipe to the remote HTTP service or the
user's connection, the size is checked again. When it is lower than the
threshold, a ``RESUME`` message is sent, and the data transfer may continue.

### remotedialer in the Rancher ecosystem

remotedialer is used to connect Rancher to the downstream clusters it manages,
enabling a user agent to access the cluster through an endpoint on the Rancher
server. remotedialer is used in three main ways:

#### Agent config and tunnel server

When the agent starts, it initially makes a client connection to the endpoint
`/v3/connect/register`, which runs an authorizer that sets some initial data
about the node. The agent continues to connect to the endpoint `/v3/connect` on
a loop. On each connection, it runs an OnConnect handler which pulls down node
configuration data from `/v3/connect/config`.

#### Steve Aggregation

The steve aggregation server on the agent establishes a remotedialer Session
with Rancher, making the steve API on the downstream cluster accessible from
the Rancher server and facilitating resource watches.

#### Health Check

The clusterconnected controller in Rancher uses the established tunnel to check
that clusters are still responsive and sets alert conditions on the cluster
object if they are not.

#### HA operation (peering)

remotedialer supports a mode where multiple servers can be configured as peers.
In that mode all servers maintain a mapping of all remotedialer client connections
to all other servers, and can route incoming requests appropriately.

Therefore, http requests referring any of the remotedialer clients can be resolved
by any of the peer servers. This is useful for high availability, and Rancher
leverages that functionality to distribute downstream clusters (running agents
acting as remotedialer clients) among replica pods (acting as remotedialer
server peers). In case one Rancher replica pod breaks down, Rancher will
reassign its downstream clusters to others.

Peers authenticate to one another via a shared token.

Running Locally
---------------

remotedialer provides an example client and server which can be run in
standalone mode FOR TESTING ONLY. These are found in the `server/` and
`client/` directories.`

### Compile

Compile the server and client:

```
make server
make client
```

### Run

Start the server first.

```
./server/server
```

The server has debug mode off by default. Enable it with `--debug`.

The client proxies requests from the remotedialer server to a web server, so it
needs to be run somewhere where it can access the web server you want to
target. The remotedialer server also needs to be reachable by the client.

For testing purposes, a basic HTTP file server is provided. Build the server with:

```
make dummy
```

Create a directory with files to serve, then run the web server from that directory:

```
mkdir www
cd www
echo 'hello' > bigfile
/path/to/dummy
```

Run the client with

```
./client/client
```

Both server and client can be run with even more verbose logging:

```
CATTLE_TUNNEL_DATA_DEBUG=true ./server/server --debug
CATTLE_TUNNEL_DATA_DEBUG=true ./client/client
```

### Usage

If the remotedialer server is running on 192.168.0.42, and the web service that
the client can access is running at address 127.0.0.1:8125, make proxied
requests like this:

```
curl http://192.168.0.42:8123/client/foo/http/127.0.0.1:8125/bigfile
```

where `foo` is the hardcoded client ID for this test server.

This test server only supports GET requests.

### HA Usage

To test remotedialer in HA mode, first start the dummy server from an appropriate directory, eg.:

```shell
cd /tmp
mkdir www
cd www
echo 'hello' > bigfile
/path/to/dummy -listen :8125
```

Then start two peer remotedialer servers with the `-peers id:token:url` flag:

```shell
./server/server -debug -id first -token aaa -listen :8123 -peers second:aaa:ws://localhost:8124/connect &
./server/server -debug -id second -token aaa -listen :8124 -peers first:aaa:ws://localhost:8123/connect
```

Then connect a client to the first server, eg:
```shell
./client/client -id foo -connect ws://localhost:8123/connect
```

Finally, use the second server to make a request to the client via the first server:
```
curl http://localhost:8124/client/foo/http/127.0.0.1:8125/
```

# Versioning

See [VERSION.md](VERSION.md).
