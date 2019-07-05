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
	"log"
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
var uribase = flag.String("ub", "gs://audioscratch", "find the audio files in this bucket path")
var language = flag.String("lang", "en-US", "language code for transcription, defaults to en-US")

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
				LanguageCode: *language,
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
				LanguageCode:               *language,
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
