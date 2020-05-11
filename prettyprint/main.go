package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1p1beta1"
)

// TODO(rjk): Update
const usage = `prettyprint`

func main() {
	flag.Parse()

	// TODO(rjk): Be able to process multiple files at once.
	for _, fn := range flag.Args() {
		doprettyprint(fn)
	}
}

func dumpwordbundles(speakers [][]*wordBundle) {
	for i, speaker := range speakers {
		log.Println("speaker", i)
		if speaker != nil {
			for i, wb := range speaker {
				log.Printf("[%d]: %v\n", i, wb)
			}
		}
	}
}

// doprettyprint will convert a single JSON transcription filename into
// something that approximates the formatting of a screenplay.
func doprettyprint(filename string) error {
	fd, err := os.Open(filename)
	if err != nil {
		log.Println("can't open input file", filename, "because", err)
		return err
	}
	defer fd.Close()
	slurper := json.NewDecoder(fd)

	var resp speechpb.LongRunningRecognizeResponse
	if err := slurper.Decode(&resp); err != nil {
		log.Printf("%s: can't decode transcription JSON file because %v\n", filename, err)
		return err
	}

	var speakers SpeakersType
	if len(resp.Results) < 1 || resp.Results[len(resp.Results)-1].Alternatives == nil {
		log.Printf("last Result in input JSON %s is empty assuming no speaker separation", filename)
	} else {
		speakers = aggregateWords(&resp)
	}

	bp := filepath.Base(filename)
	ofn := strings.TrimSuffix(bp, filepath.Ext(bp)) + ".txt"
	ofd, err := os.Create(ofn)
	if err != nil {
		log.Fatalln("can't open ouput filename", ofn, "because", err)
	}
	defer ofd.Close()
	bofd := bufio.NewWriter(ofd)

	offset := gettimeoffset(filename)

	if speakers == nil {
		if err := printTranscript(&resp, bofd); err != nil {
			log.Printf("File %s failed in printTranscript: %v\n", filename, err)
		}
	} else {
		if err := printWords(speakers, bofd, offset); err != nil {
			log.Printf("File %s failed in printWords: %v\n", filename, err)
		}
	}
	if err := bofd.Flush(); err != nil {
		log.Printf("File %s failed to flush: %v\n", filename, err)
	}
	return nil
}

func printLinebrokenString(ofd *bufio.Writer, s string) error {
	for _, r := range s {
		switch r {
		case '?', '.', '!':
			if _, err := ofd.WriteRune(r); err != nil {
				return err
			}
			if _, err := ofd.WriteRune('\n'); err != nil {
				return err
			}
		case '\n':
			if _, err := ofd.WriteRune(' '); err != nil {
				return err
			}
		default:
			if _, err := ofd.WriteRune(r); err != nil {
				return err
			}
		}
	}
	if _, err := ofd.WriteRune(' '); err != nil {
		return err
	}

	return nil
}

// printTranscript prints the transcription contents if no per-speaker
// content was available.
func printTranscript(resp *speechpb.LongRunningRecognizeResponse, ofd *bufio.Writer) error {
	log.Println("running printTranscript")
	for _, r := range resp.Results {
		// Maybe the Result is empty? Skip it.
		if len(r.Alternatives) == 0 {
			continue
		}

		if err := printLinebrokenString(ofd, r.Alternatives[0].Transcript); err != nil {
			return err
		}

		// Insert two blank lines after the end of a particular block.
		if _, err := ofd.WriteRune('\n'); err != nil {
			return err
		}
		if _, err := ofd.WriteRune('\n'); err != nil {
			return err
		}
	}
	return nil
}

type wordBundle struct {
	utterance string
	speaker   string // So that I can emit nice names.
	start     time.Duration
	end       time.Duration
}

func makeWordBundle(wi *speechpb.WordInfo) *wordBundle {
	return &wordBundle{
		utterance: wi.Word,
		speaker:   fmt.Sprintf("SPEAKER_%d", wi.SpeakerTag),
		start:     time.Duration(int64(wi.StartTime.Nanos) + int64(time.Second)*int64(wi.StartTime.Seconds)),
		end:       time.Duration(int64(wi.EndTime.Nanos) + int64(time.Second)*int64(wi.EndTime.Seconds)),
	}
}

// mergeUtterance merges nwb into wb.
func (wb *wordBundle) mergeUtterance(nwb *wordBundle) {
	wb.utterance = wb.utterance + " " + nwb.utterance
	wb.end = nwb.end
}

// shouldMerge tests if wordBundle nwb should be merged with wb based on
// a heuristic based on duration of pauses in human speech.
func (wb *wordBundle) shouldMerge(nwb *wordBundle) bool {
	// Based on heuristic: Mean from http://www.speech.kth.se/prod/publications/files/3418.pdf
	if nwb.start-wb.end < time.Millisecond*750 {
		return true
	}
	return false
}

// printSpeakerTime prints the speaker with timestamp to o.
func (wb *wordBundle) printSpeakerTime(o io.Writer, offset time.Duration) error {
	_, err := fmt.Fprintf(o, "%s: %s\n", offset+wb.start, wb.speaker)
	return err
}

type SpeakersType map[int][]*wordBundle

// aggregateWords builds a per-speaker word bundle if the resp contains
// per-speaker information (in the form of a non-nil Word key.) Returns
// nil if the resp is not multi-speaker.
func aggregateWords(resp *speechpb.LongRunningRecognizeResponse) SpeakersType {

	// by observation, the words are replicated each time. (i.e. if there are
	// multiple Results objects in resp, all words are in the last one.
	lastWords := resp.Results[len(resp.Results)-1].Alternatives[0].Words
	if lastWords == nil {
		return nil
	}

	speakers := make(SpeakersType)
	for _, wi := range lastWords {
		speaker, ok := speakers[int(wi.SpeakerTag)]
		if !ok {
			speaker = make([]*wordBundle, 0, 10)
			speakers[int(wi.SpeakerTag)] = speaker
		}
		wb := makeWordBundle(wi)

		if len(speaker) == 0 {
			speaker = append(speaker, wb)
			speakers[int(wi.SpeakerTag)] = speaker
			continue
		}

		lastwb := speaker[len(speaker)-1]
		if lastwb.shouldMerge(wb) {
			lastwb.mergeUtterance(wb)
		} else {
			speaker = append(speaker, wb)
			speakers[int(wi.SpeakerTag)] = speaker
		}
	}

	return speakers
}

func findEarliestSpeaker(speakers SpeakersType) int {
	t := int64(math.MaxInt64)
	sp := 0

	for i, s := range speakers {
		if len(s) > 0 {
			if int64(s[0].start) < t {
				t = int64(s[0].start)
				sp = i
			}
		}

	}
	return sp
}

func advanceSpeaker(speakers SpeakersType, speaker int) {
	if speakers[speaker] != nil && len(speakers[speaker]) > 0 {
		speakers[speaker] = speakers[speaker][1:]
	}
}

// print here...

func printWords(speakers SpeakersType, fd *bufio.Writer, offset time.Duration) error {
	speaker := findEarliestSpeaker(speakers)
	if speaker == 0 {
		return fmt.Errorf("printWords: speaker is wrongly 0")
	}

	if err := speakers[speaker][0].printSpeakerTime(fd, offset); err != nil {
		return err
	}
	for {
		if err := printLinebrokenString(fd, speakers[speaker][0].utterance); err != nil {
			return err
		}

		advanceSpeaker(speakers, speaker)

		nextspeaker := findEarliestSpeaker(speakers)
		if nextspeaker == 0 {
			break
		}

		if nextspeaker != speaker {
			if _, err := fd.WriteRune('\n'); err != nil {
				return err
			}
			if _, err := fd.WriteRune('\n'); err != nil {
				return err
			}
			speaker = nextspeaker
			if err := speakers[speaker][0].printSpeakerTime(fd, offset); err != nil {
				return err
			}
		}
	}
	return nil
}
