# If there is already a debugger container running, stop and remove it.
debuggerId=$(docker ps -aqf "name=debugger")

if [ ! -z $debuggerId ]; then
        docker exec $debuggerId kill-debugger.sh
	docker stop $debuggerId
	docker rm $debuggerId
fi

# Start new debug container. Mount a few directories of the $GOPATH into the container so we don't have to copy 
# things around.
docker run -d -p 80:80 -p 443:443 -p 2345:2345 --name debugger --privileged -v /go/bin:/go/bin2 -v /go/src/github.com/rancher/rancher/:/go/src/github.com/rancher/rancher -v /go/src/github.com/rancher/rancher/debug:/usr/bin/rancher rancher/rancher:debug start-debugger.sh
