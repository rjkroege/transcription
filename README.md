Tooling for performing transcription using the Google audio transcription API.

# `prepaudio`
This command converts a directory of reasonably arbitrary videos into an audio-only
format that can be sent to Google Cloud Storage (GCS). Run like this:

```
prepaudio <input> <ouput>
```

It does the following:

* uses `ffmpeg` to convert every file in *input* directory to a corresponding mono WAV format audio file in *output* (in parallel)
* Because the audio transcription
API surface has (perhaps had?) an upper bound on the size of a transcribed output (or input?) I don't recall from trying to read the protobuf code, `prepaudio` also chops files over an empirically determined safe upper bound for file duration into sub-slices (with a possibly stupid naming convention.)
* Files already prepped in *output* wlil not be converted again.

With the audio files prepped, transfer them into GCS with something like `gsutil`.

