.PHONY: all
all: protos

.PHONY: protos
protos:
	protoc --proto_path=proto --go_out=../../../../ --go_opt=module=github.com/rancher/rancher proto/*/*.proto

.PHONY: clean
clean:
	@rm -rf helmv2api/*.pb.go 2>/dev/null
