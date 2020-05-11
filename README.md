Tooling for performing transcription using the Google audio transcription API.

# `prepaudio`
This command converts a directory of reasonably arbitrary videos into
an audio-only format that can be sent to Google Cloud Storage (GCS).
Run like this:

```
prepaudio <input> <ouput>
```

It does the following:

* uses `ffmpeg` to convert every file in *input* directory to a
corresponding mono WAV format audio file in *output* (in parallel)
* Because the audio transcription API surface has (perhaps had?) an
upper bound on the size of a transcribed output (or input?) I don't
recall from trying to read the protobuf code, `prepaudio` also chops
files over an empirically determined safe upper bound for file
duration into sub-slices (with a possibly stupid naming convention.)
* Files already prepped in *output* will not be converted again.

With the audio files prepped, transfer them into GCS with something like `gsutil`.

# `transcribe`

`transcribe` queues a job to the Google audio transcription API, waits
for its completion and downloads the result as a JSON file. Run this
tool from a GCP node withe `cloud-platform` scope after enabling the
transcription API on the project. Any node type will do: the actual
transcription work runs elsewhere inside the GCP infrastructure.

Run like this:

```
transcribe [ -sp <speaker count> ]  -t <gcs url>
```

Provide a `-sp` count to attempt to recognize multiple speakers. This
will enable multiple additional features of the transcription API.
Note that speaker recognition is considered an advanced feature and
each job is more costly. Set the speaker count value to the expected
number of speakers.

# `prettyprint`

The transcription API returns a large JSON (well, probably a proto)
containing the transcription results. It is not particularly readable
for humans. The `prettyprint` generates a more human-formated output
from the JSON data. If the transcript was divided per-speaker
(spiffy!), each speaker's utterance is timestamped. Run like this:

```
prettyprint <transcript json files>
```

to generate an output text file corresponding to each input JSON.

# `statustool`

This tool dredges through a directory structure of video, audio etc. and
produces a nice CSV table that can be loaded into a spreadsheet. It's
sort of hacky and reflects a bunch of undocumented conventions that
are embedded elsewhere in the code.

```
statustool <directory structure for transcription>
```


