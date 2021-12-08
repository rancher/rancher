#!/bin/bash -e

protoc -I types/ types/drivers.proto --go_out=plugins=grpc:types
