// audio-trace reconstructs application-only audio for a real-device video.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"go-populous/internal/assets"
	"go-populous/internal/recording"
)

func main() {
	tracePath := flag.String("trace", "", "source JSONL trace copied from the application's private files")
	outputPath := flag.String("output", "", "new stereo WAV file (never overwrites an existing file)")
	start := flag.Int64("start-ns", 0, "video start as device Unix nanoseconds (0 uses the trace start)")
	duration := flag.Duration("duration", 90*time.Second, "duration of the video interval to reconstruct")
	flag.Parse()
	if err := run(*tracePath, *outputPath, *start, *duration); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Application-only audio written to %s (%s)\n", *outputPath, *duration)
}

func run(tracePath, outputPath string, start int64, duration time.Duration) error {
	if tracePath == "" || outputPath == "" {
		return fmt.Errorf("-trace and -output are required")
	}
	input, err := os.Open(tracePath)
	if err != nil {
		return err
	}
	defer input.Close()
	bundle, err := assets.LoadEmbedded()
	if err != nil {
		return err
	}
	output, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	err = recording.RenderAudioTraceWAV(output, input, bundle.SoundBank, recording.AudioTraceRenderConfig{
		StartUnixNano: start, Duration: duration,
	})
	closeErr := output.Close()
	if err != nil {
		return fmt.Errorf("rendering audio (incomplete output %s): %w", outputPath, err)
	}
	return closeErr
}
