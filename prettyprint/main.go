
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"time"

	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1p1beta1"
)

// TODO(rjk): Update
const usage = `Usage: transcribe <gcs uri>`

func main() {
	flag.Parse()

	// TODO(rjk): Better usage.
	if len(flag.Args()) != 1 {
		log.Println("prettyprint: needs a single input file to process")
	}

	// TODO(rjk): Be able to process multiple files at once.
	doprettyprint(flag.Args()[0])
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
		log.Fatalln("can't open the file", filename, "because", err)
	}
	defer fd.Close()
	slurper := json.NewDecoder(fd)

	var resp speechpb.LongRunningRecognizeResponse
	if err := slurper.Decode(&resp); err != nil {
		log.Println("can't re-read the transcription file because", err)
		return err
	}

	speakers := aggregateWords(&resp)
	printWords(speakers)

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
func (wb *wordBundle) printSpeakerTime(o io.Writer) error {
	_, err :=	fmt.Fprintf(o, "%s: %s\n", wb.start , wb.speaker)
	return err
}

type SpeakersType map[int][]*wordBundle


func aggregateWords(resp *speechpb.LongRunningRecognizeResponse) SpeakersType {
	speakers := make(SpeakersType)
	// by observation, the words are replicated each time.
	for _, wi := range resp.Results[len(resp.Results)-1].Alternatives[0].Words {
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

func printWords(speakers SpeakersType) {
	speaker := findEarliestSpeaker(speakers)
	if speaker == 0 {
		log.Fatalln("this shouldn't happens...")
	}

//	io.WriteString(os.Stdout, speakers[speaker][0].speaker)
//	io.WriteString(os.Stdout, "\n")
	speakers[speaker][0].printSpeakerTime(os.Stdout)
	for {
		io.WriteString(os.Stdout, speakers[speaker][0].utterance)
		io.WriteString(os.Stdout, "\n")

		advanceSpeaker(speakers, speaker)

		nextspeaker := findEarliestSpeaker(speakers)
		if nextspeaker == 0 {
			break
		}

		if nextspeaker != speaker {
			io.WriteString(os.Stdout, "\n")
			speaker = nextspeaker
//			io.WriteString(os.Stdout, speakers[speaker][0].speaker)
//			io.WriteString(os.Stdout, "\n")
			speakers[speaker][0].printSpeakerTime(os.Stdout)
		}

	}

}
