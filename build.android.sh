#!/bin/bash
#sudo apt-get install libgl1-mesa-dev xorg-dev
#go get github.com/ffdfgdfg/fyne-cross
#fyne-cross --targets=linux/amd64,windows/amd64,darwin/amd64 gui/npc/npc.go

go get -u fyne.io/fyne fyne.io/fyne/cmd/fyne
go mod vendor
cd vendor
cp -R * /go/src/
cd ..
rm -rf vendor
cd gui/npc
fyne package -os android -appID org.nps.client -icon ../../docs/logo.png
mv npc.apk ../../android_client.apk