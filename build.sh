#/bash/sh

if [ "$TRAVIS_OS_NAME" = "windows" ]; then
    go build -buildmode=c-shared -o npc_sdk.dll cmd\npc\sdk.go
    tar -czvf npc_sdk.tar.gz npc_sdk.dll npc_sdk.h
else
    wget https://github.com/upx/upx/releases/download/v3.95/upx-3.95-amd64_linux.tar.xz
    tar -xvf upx-3.95-amd64_linux.tar.xz
    cp upx-3.95-amd64_linux/upx ./

    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static"  ./cmd/npc/npc.go

    tar -czvf linux_amd64_client.tar.gz npc conf/npc.conf

    CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf linux_386_client.tar.gz npc conf/npc.conf

    CGO_ENABLED=0 GOOS=freebsd GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf freebsd_386_client.tar.gz npc conf/npc.conf

    CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf freebsd_amd64_client.tar.gz npc conf/npc.conf

    CGO_ENABLED=0 GOOS=freebsd GOARCH=arm go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf freebsd_arm_client.tar.gz npc conf/npc.conf

    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf linux_arm_v7_client.tar.gz npc conf/npc.conf

    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf linux_arm_v6_client.tar.gz npc conf/npc.conf

    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=5 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf linux_arm_v5_client.tar.gz npc conf/npc.conf


    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf linux_arm64_client.tar.gz npc conf/npc.conf


    CGO_ENABLED=0 GOOS=linux GOARCH=mips64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf linux_mips64_client.tar.gz npc conf/npc.conf


    CGO_ENABLED=0 GOOS=linux GOARCH=mips64le go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf linux_mips64le_client.tar.gz npc conf/npc.conf


    CGO_ENABLED=0 GOOS=linux GOARCH=mipsle go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf linux_mipsle_client.tar.gz npc conf/npc.conf


    CGO_ENABLED=0 GOOS=linux GOARCH=mips go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf linux_mips_client.tar.gz npc conf/npc.conf


    CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf win_386_client.tar.gz npc.exe conf/npc.conf


    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf win_amd64_client.tar.gz npc.exe conf/npc.conf


    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go

    tar -czvf macos_client.tar.gz npc conf/npc.conf

    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf linux_amd64_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps

    CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf linux_386_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps

    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=5 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf linux_arm_v5_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps

    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf linux_arm_v6_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps

    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf linux_arm_v7_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf linux_arm64_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


    CGO_ENABLED=0 GOOS=freebsd GOARCH=arm go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf freebsd_arm_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


    CGO_ENABLED=0 GOOS=freebsd GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf freebsd_386_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


    CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf freebsd_amd64_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps



    CGO_ENABLED=0 GOOS=linux GOARCH=mips go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf linux_mips_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


    CGO_ENABLED=0 GOOS=linux GOARCH=mips64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf linux_mips64_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


    CGO_ENABLED=0 GOOS=linux GOARCH=mips64le go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf linux_mips64le_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


    CGO_ENABLED=0 GOOS=linux GOARCH=mipsle go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf linux_mipsle_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps



    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf macos_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps


    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf win_amd64_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps.exe


    CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go

    tar -czvf win_386_server.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps.exe

    export VERSION=0.24.3
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
    sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
    sudo apt-get update
    sudo apt-get -y -o Dpkg::Options::="--force-confnew" install docker-ce
    docker --version
    git clone https://github.com/cnlh/spksrc.git ~/spksrc
    mkdir ~/spksrc/nps && cp -rf ./* ~/spksrc/nps/
    docker run -itd --name spksrc --env VERSION=$VERSION  -v ~/spksrc:/spksrc synocommunity/spksrc /bin/bash
    docker exec -it spksrc /bin/bash -c 'cd /spksrc && make setup && cd /spksrc/spk/npc && make'
    cp ~/spksrc/packages/npc_noarch-all_$VERSION-1.spk ./npc_$VERSION.spk



fi
