package main

import (
	"encoding/binary"
	"testing"
)

func TestRMS16(t *testing.T) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint16(buf[0:], uint16(int16(1000)))
	binary.LittleEndian.PutUint16(buf[2:], 0xFC18)
	if got := rms16(buf); got != 1000 {
		t.Fatalf("rms16 = %v", got)
	}
	if rms16(nil) != 0 {
		t.Fatal("empty buffer must be zero")
	}
}
