package main

import (
	"testing"
	"time"
)

func TestEdit(t *testing.T) {
	tt := []struct {
		input    string
		duration time.Duration
	}{
		{
			"_04-22-_Original-Media_Clip-,3.json",
			time.Duration(0),
		},
		{
			"_04-22-_Original-Media_Clip-,5-<0>.json",
			time.Duration(0),
		},
		{
			"_04-22-_Original-Media_Clip-,5-<s>.json",
			time.Duration(0),
		},
		{
			"_04-22-_Original-Media_Clip-,5-<1>.json",
			time.Duration(2700000000000),
		},
	}

	for i, tv := range tt {
		ot := gettimeoffset(tv.input)
		if got, want := ot, tv.duration; got != want {
			t.Errorf("%d: failed on %s: got %d, want %d\n", i, tv.input, got, want)
		}
	}
}
