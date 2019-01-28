package utils

import (
	"log"
	"testing"
)

var rate = NewRate(100 * 1024)

func TestRate_Get(t *testing.T) {
	rate.Start()
	for i := 0; i < 5; i++ {
		go test(i)
	}
	test(5)
}

func test(i int) {
	for {
		rate.Get(64 * 1024)
		log.Println("get ok", i)
	}
}
