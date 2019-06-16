package main

import (
	"testing"
)

func TestFindtime(t *testing.T) {

	in := "../../rawaudios/_April-23,-kids_Original-Media_Clip-,8.wav, WAVE, Num Tracks:     1\n----\n42.048 sec, format:   1 ch,  48000 Hz, 'lpcm' (0x0000000C) 16-bit little-endian signed integer\n"

	ov, _ :=  findtime([]byte(in))
	if want, got := 42.048, ov ; !(want + .001 > got && want - .001 < got) {
		t.Errorf("got %v but wanted %v\n", got, want)
	}
}
