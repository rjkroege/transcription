// Derived from Google example:
//
// Copyright 2016 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Command transcribe asks the Google Speech API to transcribe
// an audio file that is stored already in GCS.

package main

import (
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

	// Do we still need this? Remove later if we don't actually need it.
	"golang.org/x/net/context"

	speech "cloud.google.com/go/speech/apiv1p1beta1"
	"google.golang.org/api/option"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1p1beta1"
	"google.golang.org/grpc"
)

const usage = `Usage: transcribe <gcs uri>
`

var speakercount = flag.Int("sp", 1, "Set the number of speakers in this audio file")
var transcribe = flag.String("t", "", "transcribe the argument")

// TODO(rjk): when I have better naming, might want to conver this in some way...
var prettyprint = flag.String("pp", "transcript.json", "format and output the specified pre-existing transcription")

var uribase = flag.String("ub", "gs://audioscratch", "find the audio files in this bucket path")

var testlog = flag.Bool("testlog", false,
	"Log in the conventional way for running in a terminal.")

const dryrun = false

func LogToFile() func() {
	logFile, err := os.OpenFile("transcribe-log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Panic("leap couldn't make a logging file: %v", err)
	}

	log.SetOutput(logFile)
	return func() {
		log.SetOutput(os.Stderr)
		logFile.Close()
	}
}

func dotranscribe(shorturi string) {
	// Prep names.
	uri := *uribase + "/" + shorturi
	basename := strings.TrimSuffix(shorturi, filepath.Ext(shorturi))
	outputfile := basename + ".json"

	// Skip files already done.
	if _, err := os.Stat(outputfile); !os.IsNotExist(err) {
		log.Printf("transcription result %s exists. skipping...", outputfile)
		return
	}

	log.Printf("transcribe %s to %s with %d speakers",
		uri, outputfile, *speakercount)
	if dryrun {
		return
	}

	// Do the transcription.
	ctx := context.Background()
	client, err := speech.NewClient(ctx, option.WithGRPCDialOption(grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1<<30))))
	if err != nil {
		log.Fatal(err)
	}

	log.Println("waiting for transcription of", outputfile)
	resp, err := sendGCS(client, uri)
	if err != nil {
		log.Fatal(err)
	}

	// Output the result.
	fd, err := os.Create(outputfile)
	if err != nil {
		log.Fatalln("can't open the file", "transcript.json", "because", err)
	}
	defer fd.Close()
	saver := json.NewEncoder(fd)
	if err := saver.Encode(resp); err != nil {
		log.Println("Can't write out the response as a json because:", err)
	}
	log.Println("completed transcribing to", outputfile)
}

func main() {
	flag.Parse()
	if !*testlog {
		defer LogToFile()()
	}

	if *transcribe != "" {
		dotranscribe(*transcribe)
		return
	}

	if *prettyprint != "" {
		doprettyprint(*prettyprint)
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

func (wb *wordBundle) mergeUtterance(nwb *wordBundle) {
	wb.utterance = wb.utterance + " " + nwb.utterance
	wb.end = nwb.end
}

func (wb *wordBundle) shouldMerge(nwb *wordBundle) bool {
	// Based on heuristic: Mean from http://www.speech.kth.se/prod/publications/files/3418.pdf
	if nwb.start-wb.end < time.Millisecond*750 {
		return true
	}
	return false
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

	io.WriteString(os.Stdout, speakers[speaker][0].speaker)
	io.WriteString(os.Stdout, "\n")
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
			io.WriteString(os.Stdout, speakers[speaker][0].speaker)
			io.WriteString(os.Stdout, "\n")

		}

	}

}

func sendGCS(client *speech.Client, gcsURI string) (*speechpb.LongRunningRecognizeResponse, error) {
	ctx := context.Background()
	var req *speechpb.LongRunningRecognizeRequest

	if *speakercount == 1 {
		// Send the contents of the audio file with the encoding and
		// and sample rate information to be transcripted.
		req = &speechpb.LongRunningRecognizeRequest{
			Config: &speechpb.RecognitionConfig{
				// These are optional yes?
				//Encoding:        speechpb.RecognitionConfig_LINEAR16,
				//SampleRateHertz: 16000,
				LanguageCode:               "en-US",
			},
			Audio: &speechpb.RecognitionAudio{
				AudioSource: &speechpb.RecognitionAudio_Uri{Uri: gcsURI},
			},
		}
	} else {
		// Send the contents of the audio file with the encoding and
		// and sample rate information to be transcripted.
		req = &speechpb.LongRunningRecognizeRequest{
			Config: &speechpb.RecognitionConfig{
				// These are optional yes?
				//Encoding:        speechpb.RecognitionConfig_LINEAR16,
				//SampleRateHertz: 16000,
				LanguageCode:               "en-US",
				EnableAutomaticPunctuation: true,
				EnableSpeakerDiarization:   true,
				DiarizationSpeakerCount:    int32(*speakercount),
				Model:                      "video",
				UseEnhanced:                true,
			},
			Audio: &speechpb.RecognitionAudio{
				AudioSource: &speechpb.RecognitionAudio_Uri{Uri: gcsURI},
			},
		}
	}

	op, err := client.LongRunningRecognize(ctx, req)
	if err != nil {
		return nil, err
	}

	for {
		resp, err := op.Poll(ctx)
		switch {
		case err != nil && op.Done():
			log.Printf("%s: op failed: %v, giving up\n", gcsURI, err)
			return nil, err
		case err != nil:
			log.Printf("%s: poll errored: %v\n", gcsURI, err)
		case err == nil && !op.Done():
			log.Printf("%s: not done yet\n", gcsURI)
		case err == nil && resp != nil && op.Done():
			return resp, err
		}
		waiter := time.NewTimer(time.Second * 120)
		<-waiter.C
	}
	return nil, nil
}
