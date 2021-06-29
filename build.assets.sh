export GOPROXY=direct

sudo apt-get update
sudo apt-get install gcc-mingw-w64-i686 gcc-multilib
env GOOS=windows GOARCH=386 CGO_ENABLED=1 CC=i686-w64-mingw32-gcc go build -ldflags "-s -w -extldflags -static -extldflags -static" -buildmode=c-shared -o npc_sdk.dll cmd/npc/sdk.go
env GOOS=linux GOARCH=386 CGO_ENABLED=1 CC=gcc go build -ldflags "-s -w -extldflags -static -extldflags -static" -buildmode=c-shared -o npc_sdk.so cmd/npc/sdk.go
tar -czvf npc_sdk.tar.gz npc_sdk.dll npc_sdk.so npc_sdk.h

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static"  ./cmd/npc/npc.go

tar -czvf linux_amd64_client.tar.gz npc conf/npc.conf conf/multi_account.conf

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

tar -czvf linux_amd64_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps
