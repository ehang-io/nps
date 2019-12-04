#!/bin/bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static"  ./cmd/npc/npc.go
upx npc
tar -czvf linux_amd64_client.tar.gz npc conf/npc.conf

CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
upx npc
tar -czvf linux_386_client.tar.gz npc conf/npc.conf

CGO_ENABLED=0 GOOS=freebsd GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
upx npc
tar -czvf freebsd_386_client.tar.gz npc conf/npc.conf

CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
upx npc
tar -czvf freebsd_amd64_client.tar.gz npc conf/npc.conf

CGO_ENABLED=0 GOOS=freebsd GOARCH=arm go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
upx npc
tar -czvf freebsd_arm_client.tar.gz npc conf/npc.conf

CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
upx npc
tar -czvf linux_arm_client.tar.gz npc conf/npc.conf


CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
upx npc
tar -czvf linux_arm64_client.tar.gz npc conf/npc.conf


CGO_ENABLED=0 GOOS=linux GOARCH=mips64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
upx npc
tar -czvf linux_mips64_client.tar.gz npc conf/npc.conf


CGO_ENABLED=0 GOOS=linux GOARCH=mips64le go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
upx npc
tar -czvf linux_mips64le_client.tar.gz npc conf/npc.conf


CGO_ENABLED=0 GOOS=linux GOARCH=mipsle go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
upx npc
tar -czvf linux_mipsle_client.tar.gz npc conf/npc.conf


CGO_ENABLED=0 GOOS=linux GOARCH=mips go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
upx npc
tar -czvf linux_mips_client.tar.gz npc conf/npc.conf


CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
upx npc
tar -czvf win_386_client.tar.gz npc.exe conf/npc.conf


CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
upx npc
tar -czvf win_amd64_client.tar.gz npc.exe conf/npc.conf


CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
upx npc
tar -czvf macos_client.tar.gz npc conf/npc.conf



CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps
tar -czvf linux_amd64_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps

CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps
tar -czvf linux_386_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps

CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps
tar -czvf linux_arm_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps
tar -czvf linux_arm64_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


CGO_ENABLED=0 GOOS=freebsd GOARCH=arm go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps
tar -czvf freebsd_arm_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


CGO_ENABLED=0 GOOS=freebsd GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps
tar -czvf freebsd_386_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps
tar -czvf freebsd_amd64_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps



CGO_ENABLED=0 GOOS=linux GOARCH=mips go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps
tar -czvf linux_mips_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


CGO_ENABLED=0 GOOS=linux GOARCH=mips64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps
tar -czvf linux_mips64_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


CGO_ENABLED=0 GOOS=linux GOARCH=mips64le go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps
tar -czvf linux_mips64le_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


CGO_ENABLED=0 GOOS=linux GOARCH=mipsle go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps
tar -czvf linux_mipsle_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps



CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps
tar -czvf macos_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps.exe
tar -czvf win_amd64_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps.exe


CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
upx nps.exe
tar -czvf win_386_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps.exe

