a1: ./alertmanager -log.level=debug -storage.path=$TMPDIR/a1 -web.listen-address=:9093 -mesh.peer-id=00:00:00:00:00:01 -mesh.nickname=a -mesh.listen-address=:8001 -config.file=examples/ha/alertmanager.yaml
a2: ./alertmanager -log.level=debug -storage.path=$TMPDIR/a2 -web.listen-address=:9094 -mesh.peer-id=00:00:00:00:00:02 -mesh.nickname=b -mesh.listen-address=:8002 -mesh.peer=127.0.0.1:8001 -config.file=examples/ha/alertmanager.yaml
a3: ./alertmanager -log.level=debug -storage.path=$TMPDIR/a3 -web.listen-address=:9095 -mesh.peer-id=00:00:00:00:00:03 -mesh.nickname=c -mesh.listen-address=:8003 -mesh.peer=127.0.0.1:8001 -config.file=examples/ha/alertmanager.yaml
wh: go run ./examples/webhook/echo.go

