package kcp

import (
	"math/rand"
	"testing"
)

func BenchmarkFECDecode(b *testing.B) {
	const dataSize = 10
	const paritySize = 3
	const payLoad = 1500
	decoder := newFECDecoder(1024, dataSize, paritySize)
	b.ReportAllocs()
	b.SetBytes(payLoad)
	for i := 0; i < b.N; i++ {
		if rand.Int()%(dataSize+paritySize) == 0 { // random loss
			continue
		}
		var pkt fecPacket
		pkt.seqid = uint32(i)
		if i%(dataSize+paritySize) >= dataSize {
			pkt.flag = typeFEC
		} else {
			pkt.flag = typeData
		}
		pkt.data = make([]byte, payLoad)
		decoder.decode(pkt)
	}
}

func BenchmarkFECEncode(b *testing.B) {
	const dataSize = 10
	const paritySize = 3
	const payLoad = 1500

	b.ReportAllocs()
	b.SetBytes(payLoad)
	encoder := newFECEncoder(dataSize, paritySize, 0)
	for i := 0; i < b.N; i++ {
		data := make([]byte, payLoad)
		encoder.encode(data)
	}
}
