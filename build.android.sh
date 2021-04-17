#/bin/bash

cd /go
apt-get install libegl1-mesa-dev libgles2-mesa-dev libx11-dev xorg-dev -y
go get -u fyne.io/fyne/v2/cmd/fyne fyne.io/fyne/v2
#mkdir -p /go/src/fyne.io
#cd src/fyne.io
#git clone https://github.com/fyne-io/fyne.git
#cd fyne
#git checkout v1.2.0
#go install -v ./cmd/fyne
#fyne package -os android fyne.io/fyne/cmd/hello
echo "fyne install success"
mkdir -p /go/src/ehang.io/nps
cp -R /app/* /go/src/ehang.io/nps
cd /go/src/ehang.io/nps
#go get -u fyne.io/fyne fyne.io/fyne/cmd/fyne
rm cmd/npc/sdk.go
#go get -u ./...
#go mod tidy
#rm -rf /go/src/golang.org/x/mobile
echo "tidy success"
cd /go/src/ehang.io/nps
go mod vendor
cd vendor
cp -R * /go/src
cd ..
rm -rf vendor
#rm -rf ~/.cache/*
echo "vendor success"
cd gui/npc
fyne package -appID org.nps.client -os android -icon ../../docs/logo.png
mv npc.apk /app/android_client.apk
echo "android build success"
