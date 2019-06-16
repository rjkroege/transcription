package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/codeskyblue/go-sh"
	"github.com/gammazero/workerpool"
)

const helptext = `Usage: prepaudio indir outdir

prepaudio reads all of the media files in indir and converts them into
WAV format audio not exceeding the maximum supported length in outdir.
`

// usage prints a usage message for this command.
func usage(status int) {
	io.WriteString(os.Stdout, helptext)
	os.Exit(status)
}

func main() {
	log.Println("Starting conversion")
	flag.Parse()

	indir := flag.Arg(0)
	outdir := flag.Arg(1)

	if indir == "" {
		log.Println("No indir specified")
		usage(1)
	}
	if outdir == "" {
		log.Println("No outdir specified")
		usage(1)
	}

	// Enumerate files in indir. Some may not be convertible. Collect the
	// issues and dump that later.
	infilenames, err := filepath.Glob(filepath.Join(indir, "*"))
	if err != nil {
		log.Println("Error reading files from indir: ", err)
		usage(1)
	}

	// Enumerate files in outdir and put in a hash.
	outfilenames, err := filepath.Glob(filepath.Join(outdir, "*.wav"))
	if err != nil {
		log.Println("Error reading files from outdir: ", err)
		usage(1)
	}
	outfilemap := make(map[string]struct{}, len(outfilenames))
	for _, o := range outfilenames {
		outfilemap[o] = struct{}{}
	}

	// Some of the out files might be slices of a larger file. In which case,
	// we have a problem here. Becuase we don't know that yet. We'll have to
	// fix that up later. We'll want to preserve that state in some fashion
	// between runs. Or just not care.

	// Setup the worker pool.
	wp := workerpool.New(4)
	donez := make(chan string)
	filezcount := 0

	for _, ofn := range infilenames {
		fn := ofn
		bon := filepath.Base(fn)
		bonexed := strings.TrimSuffix(bon, filepath.Ext(bon))
		destname := filepath.Join(outdir, bonexed+".wav")
		slicedname := makeslicename(outdir, bonexed, 0)

		if _, ok := outfilemap[slicedname]; ok {
			// We have a slice for this output. So skip. If you want to recreate
			// slices, be sure to delete them all by hand before hand.
			continue
		}

		if _, ok := outfilemap[destname]; !ok {
			wp.Submit(func() {
				convertandsplit(fn, outdir, bonexed, destname, wp, donez)
			})
			filezcount++
		}
	}

	// Wait for every per-input file to have launched and submitted its work.
	temptodelete := make([]string, 0, 10)
	for ; filezcount > 0; filezcount-- {
		v := <-donez
		if v != "" {
			temptodelete = append(temptodelete, v)
		}
	}

	// Wait for all the workers to take the day off.
	wp.StopWait()

	// Clean up the pre-split files.
	for _, fn := range temptodelete {
		if err := os.Remove(fn); err != nil {
			log.Printf("Can't remove %s: %v\n", fn, err)
		}
	}
	log.Println("Done!")
}

// convertandsplit converts the audio files and splite them as needed
// into duration limited pieces.
func convertandsplit(fn, outdir, bonexed, destname string, wp *workerpool.WorkerPool, donez chan<- string) {
	// log.Printf("Starting conversion of %s -> %s\n", fn, destname)

	// Runs ffmpeg -i infile -ac 1 outfile.wav, the -ac 1 forces down-mix to mono.
	if ffmpegoutput, err := sh.Command("ffmpeg", "-i", fn, "-ac", "1", destname).CombinedOutput(); err != nil {
		log.Printf("command failed %v\nLog for conversion of %s -> %s\n%s", err, fn, destname, string(ffmpegoutput))
		donez <- ""
		return
	}

	dur, err := runavinfo(destname)
	if err != nil {
		log.Printf("can't duration test %s: %v", destname, err)
		donez <- ""
		return
	}

	// log.Printf("duration %s: %v\n", destname, dur)
	if dur < 3000 {
		donez <- ""
		return
	}

	// log.Printf("Finished conversion of %s -> %s but must split\n", fn, destname)
	for i := 0; float64(i)*2700 < dur; i++ {
		ii := i
		wp.Submit(func() {
			runsplit(destname, outdir, bonexed, ii)
		})
	}
	donez <- destname
}

// makeslicename creates the special filenames for slices of a larger wav.
func makeslicename(outdir, bonexed string, i int) string {
	return filepath.Join(outdir, fmt.Sprintf("%s-〖%d〗.wav", bonexed, i))
}

// runsplit divides wav file destname into chunks small enough to work
// with the Google transcription service.
func runsplit(destname, outdir, bonexed string, i int) {
	slicename := makeslicename(outdir, bonexed, i)
	// log.Println("slicing", destname, "to",  slicename)

	// Runs ffmpeg -ss <start> -t <length> -i <infile>  <outfile>
	if out, err := sh.Command("ffmpeg", "-ss", fmt.Sprintf("%d", i*2700), "-t", fmt.Sprintf("%d", 3000), "-i", destname, slicename).CombinedOutput(); err != nil {
		log.Printf("command failed %v\nLog for slicing of %s -> %s\n%s", err, destname, slicename, string(out))
		return
	}
	// log.Println("finished slice", slicename)
}

// runavinfo gets the duration of destname in seconds.
// TODO(rjk): Give it a better name.
func runavinfo(destname string) (float64, error) {
	// TODO(rjk): Could use a native library for parsing wav files here?
	cmdout, err := sh.Command("afinfo", "-b", destname).Output()
	if err != nil {
		log.Printf("Can't extract info about dest %s: %v\n", destname, err)
		return 0.0, err
	}
	return findtime(cmdout)
}
