Reverse Tunneling Dialer
========================

Client makes an outbound connection to a server.  The server can now do net.Dial from the
server that will actually do a net.Dial on the client and pipe all bytes back and forth.

Fun times!

Refer to [`server/`](server/) and [`client/`](client/) how to use.  Or don't.... This framework can hurt your head
trying to conceptualize.

See also:

* [inlets.dev](https://inlets.dev) which uses the client and server components to form a tunnel for clients behind NAT or firewalls.
