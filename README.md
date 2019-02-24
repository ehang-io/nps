#!/bin/bash
cp ./nps/conf/npc.conf ./

GOOS=linux GOARCH=amd64 go build -ldflags "-s -w"  ./nps/cmd/npc/npc.go
upx npc
tar -czvf linux_amd64_client.tar.gz npc npc.conf 

GOOS=linux GOARCH=386 go build -ldflags "-s -w" nps/cmd/npc/npc.go
upx npc
tar -czvf linux_386_client.tar.gz npc npc.conf

GOOS=linux GOARCH=arm go build -ldflags "-s -w" nps/cmd/npc/npc.go
upx npc
tar -czvf linux_arm_client.tar.gz npc npc.conf

GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" nps/cmd/npc/npc.go
upx npc
tar -czvf linux_arm64_client.tar.gz npc npc.conf

GOOS=linux GOARCH=mips64 go build -ldflags "-s -w" nps/cmd/npc/npc.go
upx npc
tar -czvf linux_mips64_client.tar.gz npc npc.conf


GOOS=linux GOARCH=mips64le go build -ldflags "-s -w" nps/cmd/npc/npc.go
upx npc
tar -czvf linux_mips64le_client.tar.gz npc npc.conf

GOOS=linux GOARCH=mipsle go build -ldflags "-s -w" nps/cmd/npc/npc.go
upx npc
tar -czvf linux_mipsle_client.tar.gz npc npc.conf

CGO_ENABLED=0 GOOS=linux GOARCH=mips go build -ldflags "-s -w" nps/cmd/npc/npc.go
upx npc
tar -czvf linux_mips_client.tar.gz npc npc.conf

GOOS=windows GOARCH=386 go build -ldflags "-s -w" nps/cmd/npc/npc.go
upx npc.exe
tar -czvf win_386_client.tar.gz npc npc.conf.exe

GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" nps/cmd/npc/npc.go
upx npc.exe
tar -czvf win_amd64_client.tar.gz npc npc.conf.exe

go build -ldflags "-s -w" nps/cmd/npc/npc.go
upx npc
tar -czvf macos_client.tar.gz npc npc.conf


cd /Users/liuhe/go/src/github.com/cnlh/nps
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" ./cmd/nps/nps.go
upx nps
cd /Users/liuhe/go/src/github.com/cnlh
tar -czvf linux_amd64_server.tar.gz nps/conf/nps.conf nps/conf/clients.csv nps/conf/hosts.csv nps/conf/tasks.csv nps/web/views nps/web/static nps/nps

cd /Users/liuhe/go/src/github.com/cnlh/nps
GOOS=linux GOARCH=386 go build -ldflags "-s -w" ./cmd/nps/nps.go
upx nps
cd /Users/liuhe/go/src/github.com/cnlh
tar -czvf linux_386_server.tar.gz nps/conf/nps.conf nps/conf/clients.csv nps/conf/hosts.csv nps/conf/tasks.csv nps/web/views nps/web/static nps/nps

cd /Users/liuhe/go/src/github.com/cnlh/nps
CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -ldflags "-s -w" ./cmd/nps/nps.go
upx nps
cd /Users/liuhe/go/src/github.com/cnlh
tar -czvf linux_arm_server.tar.gz nps/conf/nps.conf nps/conf/clients.csv nps/conf/hosts.csv nps/conf/tasks.csv nps/web/views nps/web/static nps/nps

cd /Users/liuhe/go/src/github.com/cnlh/nps
GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" ./cmd/nps/nps.go
upx nps
cd /Users/liuhe/go/src/github.com/cnlh
tar -czvf linux_arm64_server.tar.gz nps/conf/nps.conf nps/conf/clients.csv nps/conf/hosts.csv nps/conf/tasks.csv nps/web/views nps/web/static nps/nps

cd /Users/liuhe/go/src/github.com/cnlh/nps
GOOS=linux GOARCH=mips go build -ldflags "-s -w" ./cmd/nps/nps.go
upx nps
cd /Users/liuhe/go/src/github.com/cnlh
tar -czvf linux_mips_server.tar.gz nps/conf/nps.conf nps/conf/clients.csv nps/conf/hosts.csv nps/conf/tasks.csv nps/web/views nps/web/static nps/nps

cd /Users/liuhe/go/src/github.com/cnlh/nps
CGO_ENABLED=0 GOOS=linux GOARCH=mips64 go build -ldflags "-s -w" ./cmd/nps/nps.go
upx nps
cd /Users/liuhe/go/src/github.com/cnlh
tar -czvf linux_mips64_server.tar.gz nps/conf/nps.conf nps/conf/clients.csv nps/conf/hosts.csv nps/conf/tasks.csv nps/web/views nps/web/static nps/nps

cd /Users/liuhe/go/src/github.com/cnlh/nps
GOOS=linux GOARCH=mips64le go build -ldflags "-s -w" ./cmd/nps/nps.go
upx nps
cd /Users/liuhe/go/src/github.com/cnlh
tar -czvf linux_mips64le_server.tar.gz nps/conf/nps.conf nps/conf/clients.csv nps/conf/hosts.csv nps/conf/tasks.csv nps/web/views nps/web/static nps/nps

cd /Users/liuhe/go/src/github.com/cnlh/nps
GOOS=linux GOARCH=mipsle go build -ldflags "-s -w" ./cmd/nps/nps.go
upx nps
cd /Users/liuhe/go/src/github.com/cnlh
tar -czvf linux_mipsle_server.tar.gz nps/conf/nps.conf nps/conf/clients.csv nps/conf/hosts.csv nps/conf/tasks.csv nps/web/views nps/web/static nps/nps


cd /Users/liuhe/go/src/github.com/cnlh/nps
go build -ldflags "-s -w" ./cmd/nps/nps.go
upx nps
cd /Users/liuhe/go/src/github.com/cnlh
tar -czvf macos_server.tar.gz nps/conf/nps.conf nps/conf/clients.csv nps/conf/hosts.csv nps/conf/tasks.csv nps/web/views nps/web/static nps/nps

rm /Users/liuhe/go/src/github.com/cnlh/nps/nps


cd /Users/liuhe/go/src/github.com/cnlh/nps
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" ./cmd/nps/nps.go
upx nps.exe
cd /Users/liuhe/go/src/github.com/cnlh
tar -czvf win_amd64_server.tar.gz nps/conf/nps.conf nps/conf/clients.csv nps/conf/hosts.csv nps/conf/tasks.csv nps/web/views nps/web/static nps/nps.exe

cd /Users/liuhe/go/src/github.com/cnlh/nps
GOOS=windows GOARCH=386 go build -ldflags "-s -w" ./cmd/nps/nps.go
upx nps.exe
cd /Users/liuhe/go/src/github.com/cnlh
tar -czvf win_386_server.tar.gz nps/conf/nps.conf nps/conf/clients.csv nps/conf/hosts.csv nps/conf/tasks.csv nps/web/views nps/web/static nps/nps.exe

rm /Users/liuhe/go/src/github.com/cnlh/nps/nps.exe
