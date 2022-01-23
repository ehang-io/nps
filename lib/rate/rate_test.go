package rate

import (
	"math"
	"testing"
	"time"
)

func TestRate_GetWrite(t *testing.T) {
	r := NewRate(1024)
	r.Write(2048)
	n := 0
	go func() {
		for {
			r.Get(1024)
			n += 1024
		}
	}()
	select {
	case <-time.After(time.Second):
		r.Stop()
		if n != 2048 {
			t.Fatal("get token error", n)
		}
	}
}

func TestRate_StartGetRate(t *testing.T) {
	r := NewRate(1024)
	r.Start()
	n := 0
	go func() {
		for {
			err := r.Get(1024)
			if err != nil {
				return
			}
			n += 1024
		}
	}()
	select {
	case <-time.After(time.Second * 5):
		r.Stop()
		time.Sleep(time.Second * 2)
		if n < 4*1024 || n > 1024*5 {
			t.Fatal("get error", n)
		}
		if math.Abs(float64(r.GetNowRate()-1024)) > 100 {
			t.Fatal("rate error", n)
		}
	}
}
