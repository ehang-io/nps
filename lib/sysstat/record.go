// Package sysstat is the host stats a client rides on its mux pings: CPU and
// memory use and each GPU's power, in a fixed-length record, and the server's
// short history of them.
package sysstat

import (
	nps_mux "ehang.io/nps-mux"
)

// MaxGPU is how many GPUs a record has slots for.
const MaxGPU = 8

// Record layout, nps_mux.StatsLen bytes:
//
//	0       marker, nps_mux.StatsMagic
//	1       GPU count (the real one, even above MaxGPU)
//	2       CPU use, in 0.5%
//	3       memory use, in 0.5%
//	4..12   each GPU's power now, in 4W
//	12..20  each GPU's power limit, in 4W
const (
	offCount = 1
	offCPU   = 2
	offMem   = 3
	offPower = 4
	offLimit = offPower + MaxGPU
)

func init() {
	if offLimit+MaxGPU != nps_mux.StatsLen {
		panic("sysstat: record layout does not match nps_mux.StatsLen")
	}
}

// Stats is a decoded record.
type Stats struct {
	CPU, Mem float64 // percent
	GPUs     int     // how many the host has, possibly more than Power holds
	Power    []float64
	Limit    []float64 // watts, one per slot of Power
}

// Sample is what Collect reads; the zero GPU fields mean none.
type Sample struct {
	CPU, Mem float64 // percent
	GPUs     []GPU
}

// GPU's power in watts. Zero Limit means unknown.
type GPU struct {
	Power, Limit float64
}

func quant(v, unit float64, max int) byte {
	n := int(v/unit + 0.5)
	if n < 0 {
		n = 0
	}
	if n > max {
		n = max
	}
	return byte(n)
}

// Encode packs a sample into a record.
func Encode(s Sample) []byte {
	b := make([]byte, nps_mux.StatsLen)
	b[0] = nps_mux.StatsMagic
	n := len(s.GPUs)
	if n > 255 {
		n = 255
	}
	b[offCount] = byte(n)
	b[offCPU] = quant(s.CPU, 0.5, 200)
	b[offMem] = quant(s.Mem, 0.5, 200)
	for i := 0; i < len(s.GPUs) && i < MaxGPU; i++ {
		b[offPower+i] = quant(s.GPUs[i].Power, 4, 255)
		b[offLimit+i] = quant(s.GPUs[i].Limit, 4, 255)
	}
	return b
}

// Decode unpacks a record; ok is false for anything else.
func Decode(b []byte) (s Stats, ok bool) {
	if len(b) != nps_mux.StatsLen || b[0] != nps_mux.StatsMagic {
		return
	}
	s.GPUs = int(b[offCount])
	s.CPU = float64(b[offCPU]) / 2
	s.Mem = float64(b[offMem]) / 2
	n := s.GPUs
	if n > MaxGPU {
		n = MaxGPU
	}
	for i := 0; i < n; i++ {
		s.Power = append(s.Power, float64(b[offPower+i])*4)
		s.Limit = append(s.Limit, float64(b[offLimit+i])*4)
	}
	return s, true
}
