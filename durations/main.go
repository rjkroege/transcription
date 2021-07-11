package main

import (
	"log"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Duration struct {
	SourceFile string
	TrackDuration string
}


func main() {
	flag.Parse()
	fname := flag.Arg(0)

	// 1. Read file
	fd, err := os.Open(fname)
	if err != nil {
		log.Fatalf("can't open %q: %v", fname, err)
	}
	
	// 2. parse
	decoder := json.NewDecoder(fd)
	clips := make([]Duration, 0)
	if err := decoder.Decode(&clips); err != nil {
		log.Fatalln("can't decode", err)
	}

	sum := 0.
	for _, p := range clips {
		sum += timeinminutes(p.TrackDuration)
		fmt.Printf("%s	%s\n", p.SourceFile, p.TrackDuration)
	}

	fmt.Printf("totaltime: %.2f minutes\n", sum / 60.)

}


func timeinminutes(timevalue string) float64 {
	if strings.HasSuffix(timevalue, " s") {
		fstring := strings.TrimSuffix(timevalue, " s")
		f, err := strconv.ParseFloat(fstring, 64)
		if err != nil {
			log.Fatalf("can't parse %s: %v", timevalue, err)
		}
		return f
	} 


	times := strings.Split(timevalue, ":")
	seconds := 0.
	m := 3600.
	for _, tvs := range times {
		f, err := strconv.ParseFloat(tvs, 64)
		if err != nil {
			log.Fatalf("can't parse %s in %s: %v", tvs,timevalue, err)
		}
		seconds += m * f
		m = m / 60.
	}
	
	return seconds
}
