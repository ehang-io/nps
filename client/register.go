package client

import (
	"encoding/binary"
	"log"
	"os"

	"ehang.io/nps/lib/common"
)

func RegisterLocalIp(server string, vKey string, tp string, proxyUrl string, hour int) {
	c, err := NewConn(tp, vKey, server, common.WORK_REGISTER, proxyUrl)
	if err != nil {
		log.Fatalln(err)
	}
	if err := binary.Write(c, binary.LittleEndian, int32(hour)); err != nil {
		log.Fatalln(err)
	}
	log.Printf("Successful ip registration for local public network, the validity period is %d hours.", hour)
	os.Exit(0)
}
