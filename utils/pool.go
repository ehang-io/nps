package utils

import (
	"sync"
)

const poolSize = 64 * 1024
const poolSizeSmall = 100
const poolSizeUdp = 1472
const poolSizeCopy = 32 * 1024

var BufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, poolSize)
	},
}

var BufPoolUdp = sync.Pool{
	New: func() interface{} {
		return make([]byte, poolSizeUdp)
	},
}
var BufPoolMax = sync.Pool{
	New: func() interface{} {
		return make([]byte, poolSize)
	},
}
var BufPoolSmall = sync.Pool{
	New: func() interface{} {
		return make([]byte, poolSizeSmall)
	},
}
var BufPoolCopy = sync.Pool{
	New: func() interface{} {
		return make([]byte, poolSizeCopy)
	},
}

func PutBufPoolCopy(buf []byte) {
	if cap(buf) == poolSizeCopy {
		BufPoolCopy.Put(buf[:poolSizeCopy])
	}
}
