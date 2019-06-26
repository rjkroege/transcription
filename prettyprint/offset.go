package main

import (
	"regexp"
	"strconv"
	"time"
)

const sliceregex = `<([0-9]+)>`

var fnripper *regexp.Regexp

func init() {
	fnripper = regexp.MustCompile(sliceregex)
}

// gettimeoffset extracts the slice offset substring from the filename
// and computes the corresponding time offset.
func gettimeoffset(fn string) time.Duration {
	matches := fnripper.FindAllStringSubmatch(fn, -1)

	if len(matches) < 1 || len(matches[0]) < 2 {
		return time.Duration(0)
	}

	if s, err := strconv.ParseInt(matches[0][1], 10, 64); err == nil {
		return time.Duration(s * 2700) * time.Second
	}

	return time.Duration(0)
}
