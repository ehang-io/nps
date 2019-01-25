package utils

import "sync"

const poolSize = 64 * 1024
const poolSizeSmall = 100
const poolSizeUdp = 1472
const poolSizeCopy = 32 * 1024

var bufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, poolSize)
	},
}
var BufPoolUdp = sync.Pool{
	New: func() interface{} {
		return make([]byte, poolSizeUdp)
	},
}
var bufPoolMax = sync.Pool{
	New: func() interface{} {
		return make([]byte, poolSize)
	},
}
var bufPoolSmall = sync.Pool{
	New: func() interface{} {
		return make([]byte, poolSizeSmall)
	},
}
var bufPoolCopy = sync.Pool{
	New: func() interface{} {
		return make([]byte, poolSizeCopy)
	},
}
