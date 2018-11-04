package main

import (
	"crypto/sha1"
	"time"
)

// 简单的一个校验值
func getverifyval() []byte {
	b := sha1.Sum([]byte(time.Now().Format("2006-01-02 15") + *verifyKey))
	return b[:]
}
