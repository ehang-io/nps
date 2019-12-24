#/bin/bash
#sudo apt-get install libgl1-mesa-dev xorg-dev
#go get github.com/ffdfgdfg/fyne-cross
#fyne-cross --targets=linux/amd64,windows/amd64,darwin/amd64 gui/npc/npc.go

cd /go
go get -u fyne.io/fyne fyne.io/fyne/cmd/fyne

mkdir -p /go/src/github.com/cnlh/nps
cp -R /app/* /go/src/github.com/cnlh/nps
cd /go/src/github.com/cnlh/nps
go mod vendor
cd vendor
cp -R * /go/src
cd ..
rm -rf vendor
#rm -rf ~/.cache/*
cd gui/npc
#export ANDROID_NDK_HOME=/usr/local/android_sdk/ndk-bundle
fyne package -appID org.nps.client -os android -icon ../../docs/logo.png
mv npc.apk /app/android_client.apk
