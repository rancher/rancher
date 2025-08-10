all: client dummy server

client:
	go build -o client/client ./client

dummy:
	go build -o dummy/dummy ./dummy

server:
	go build -o server/server ./server

test:
	go test -cover ./...

.PHONY: all client dummy server test
