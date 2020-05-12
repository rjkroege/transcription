package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var dirroot = flag.String("root", ".", "directories of media should be relative to this")
var ofile = flag.String("of", "output.csv", "status file to create")

const outputlayout = "2006/01/02"

type Row struct {
	Original     string
	OriginalTime string
	Audio        string
	AudioTime    string
	Json         string
	JsonTime     string
	Text         string
	TextTime     string
}

func main() {
	flag.Parse()

	oglob := filepath.Join(*dirroot, "originals", "*.mov")
	originals, err := filepath.Glob(oglob)
	if err != nil {
		log.Fatalf("can't glob %s: %v", oglob, err)
	}

	statusmap := make(map[string]*Row)
	for _, of := range originals {
		bp := filepath.Base(of)

		fi, err := os.Stat(of)
		if err != nil {
			log.Fatalf("%s can't stat: %v\n", of, err)
		}

		statusmap[bp] = &Row{
			Original:     bp,
			OriginalTime: fi.ModTime().Format(outputlayout),
		}
	}

	aglob := filepath.Join(*dirroot, "rawaudios", "*.wav")
	audios, err := filepath.Glob(aglob)
	if err != nil {
		log.Fatalf("can't glob %s: %v", aglob, err)
	}

	audiomap := make(map[string]*Row)
	// I should be able to have a trace from the result to the component
	for _, af := range audios {
		fi, err := os.Stat(af)
		if err != nil {
			log.Fatalf("%s can't stat: %v\n", af, err)
		}

		bp := filepath.Base(af)
		rp := strings.TrimSuffix(bp, filepath.Ext(bp))
		mp := rp + ".mov"
		// log.Println(bp, rp, mp)

		var v *Row
		ok := false

		if v, ok = statusmap[mp]; !ok {
			// naming pattern is dumb: -<%d>
			i := 0
			for ; i < 10; i++ {
				mp = strings.TrimSuffix(rp, fmt.Sprintf("-<%d>", i)) + ".mov"
				// log.Println(bp, rp, mp)
				if v, ok = statusmap[mp]; ok {
					break
				}

			}
			if i >= 10 {
				log.Fatalf("audio %s has no video?\n", af)
			}
		}

		audiomap[bp] = &Row{
			Original:     v.Original,
			OriginalTime: v.OriginalTime,
			Audio:        bp,
			AudioTime:    fi.ModTime().Format(outputlayout),
		}
	}

	jsonmap := mapmaker("jsons", ".json", ".wav", audiomap, func(v *Row, p, d string) *Row {
		newv := *v
		newv.Json = p
		newv.JsonTime = d
		return &newv
	})
	textmap := mapmaker("texts", ".txt", ".json", jsonmap, func(v *Row, p, d string) *Row {
		newv := *v
		newv.Text = p
		newv.TextTime = d	
		return &newv
	})
	newtextmap := mapmaker("newtexts", ".txt", ".json", jsonmap, func(v *Row, p, d string) *Row {
		newv := *v
		newv.Text = p
		newv.TextTime = d
		return &newv
	})

	// There is considerable opportunity to improve this. See note at Github.

	// Handle original texts
	outputtable := make([][]string, 0)
	for _, f := range textmap {
		outputtable = append(outputtable, converter(f))
		delete(jsonmap, f.Json)
		delete(audiomap, f.Audio)
		delete(statusmap, f.Original)
	}

	// Handle newtexts
	for _, f := range newtextmap {
		outputtable = append(outputtable, converter(f))
		delete(jsonmap, f.Json)
		delete(audiomap, f.Audio)
		delete(statusmap, f.Original)
	}

	for _, f := range jsonmap {
		outputtable = append(outputtable, converter(f))
		delete(audiomap, f.Audio)
		delete(statusmap, f.Original)
	}
	for _, f := range audiomap {
		outputtable = append(outputtable, converter(f))
		delete(statusmap, f.Original)
	}
	for _, f := range statusmap {
		outputtable = append(outputtable, converter(f))
	}

	// Sort it
	sort.Sort(ColumnZero(outputtable))

	outputtable = append([][]string{{"movie", "movie date", "audio", "audio date", "json", "json date", "text", "text date"}}, outputtable...)

	ofd, err := os.Create(*ofile)
	if err != nil {
		log.Fatalf("can't make output:", err)
	}
	owr := csv.NewWriter(ofd)
	if err := owr.WriteAll(outputtable); err != nil {
		log.Fatalf("can't write output:", err)
	}
	ofd.Close()
}

type ColumnZero [][]string

func (a ColumnZero) Len() int           { return len(a) }
func (a ColumnZero) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ColumnZero) Less(i, j int) bool { return a[i][0] < a[j][0] }

type Updater func(v *Row, path, data string) *Row

func mapmaker(path, ext, pext string, pmap map[string]*Row, vupdater Updater) map[string]*Row {
	globpath := filepath.Join(*dirroot, path, "*"+ext)
	globbers, err := filepath.Glob(globpath)
	if err != nil {
		log.Fatalf("can't glob %s: %v", globpath, err)
	}

	fmap := make(map[string]*Row)
	for _, jf := range globbers {
		bp := filepath.Base(jf)
		rp := strings.TrimSuffix(bp, filepath.Ext(bp))
		rpa := strings.TrimSuffix(rp, "-en-AU") + pext
		rp = rp + pext

		fi, err := os.Stat(jf)
		if err != nil {
			log.Fatalf("%s can't stat: %v\n", jf, err)
		}

		// log.Println(bp, rp, rpa)
		if v, ok := pmap[rp]; ok {
			fmap[bp] = vupdater(v, bp, fi.ModTime().Format(outputlayout))
		} else if v, ok := pmap[rpa]; ok {
			log.Println(rp, rpa, bp)
			fmap[bp] = vupdater(v, bp, fi.ModTime().Format(outputlayout))
		} else {
			v := &Row{}
			fmap[bp] = vupdater(v, bp, fi.ModTime().Format(outputlayout))
		}
	}
	return fmap
}

func converter(f *Row) []string {
	return []string{
		f.Original,
		f.OriginalTime,
		f.Audio,
		f.AudioTime,

		f.Json,
		f.JsonTime,
		f.Text,
		f.TextTime,
	}
}

func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}
