package main

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

func findtime(cmdout []byte) (float64, error) {
	buffy := bytes.NewBuffer(cmdout)
	scanner := bufio.NewScanner(buffy)
	thirdline := ""
	c := 0
	for scanner.Scan() {
		if c > 1 {
			thirdline = scanner.Text()
			break
		}
		c++
	}
	if err := scanner.Err(); err != nil {
		return 0.0, err
	}
	return strconv.ParseFloat(strings.SplitN(thirdline, " ", 2)[0], 64)
}
