package recording

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func ffmpegForTest(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is not installed")
	}
	return path
}

func frameRGBA(width, height int) []byte {
	frame := make([]byte, width*height*4)
	for i := 0; i < len(frame); i += 4 {
		frame[i] = 220
		frame[i+1] = 60
		frame[i+2] = 40
		frame[i+3] = 255
	}
	return frame
}

func TestRecorderMP4ContainsVideoAndDirectAudio(t *testing.T) {
	ffmpeg := ffmpegForTest(t)
	path := filepath.Join(t.TempDir(), "recordings", "demo.mp4")
	bank := testMusicBank()
	bank.Sequence = nil
	bank.Patches[0].Length = SampleRate
	bank.Patches[0].Period = 81
	bank.Samples[0] = make([]byte, SampleRate)
	for i := range bank.Samples[0] {
		bank.Samples[0][i] = byte(int8(80 * math.Sin(float64(i)*2*math.Pi/100)))
	}
	r, err := New(Config{Path: path, Width: 8, Height: 4, OutputWidth: 32, OutputHeight: 32, FFmpegPath: ffmpeg}, bank)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Abort()
	frame := frameRGBA(8, 4)
	for i := 0; i < FramesPerSecond; i++ {
		var events []int
		if i == 0 {
			events = []int{0}
		}
		if err := r.WriteFrame(frame, events, 0, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := os.Stat(r.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary recording directory still exists: %v", err)
	}
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Log("FFmpeg integration passed; ffprobe is unavailable for stream inspection")
		return
	}
	data, err := exec.Command(probe, "-v", "error", "-show_streams", "-show_format", "-of", "json", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	var info struct {
		Streams []struct {
			CodecName  string `json:"codec_name"`
			CodecType  string `json:"codec_type"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			PixFmt     string `json:"pix_fmt"`
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
			Frames     string `json:"nb_frames"`
		}
		Format struct {
			Duration string `json:"duration"`
		}
	}
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatal(err)
	}
	if len(info.Streams) != 2 {
		t.Fatalf("MP4 streams = %d, want one video and one audio", len(info.Streams))
	}
	v, a := info.Streams[0], info.Streams[1]
	if v.CodecType != "video" || v.CodecName != "h264" || v.Width != 32 || v.Height != 32 || v.PixFmt != "yuv420p" || v.Frames != "30" {
		t.Fatalf("unexpected video stream: %+v", v)
	}
	if a.CodecType != "audio" || a.CodecName != "aac" || a.Channels != 2 || a.SampleRate != "44100" {
		t.Fatalf("unexpected audio stream: %+v", a)
	}
	duration, _ := strconv.ParseFloat(info.Format.Duration, 64)
	if math.Abs(duration-1) > 0.04 {
		t.Fatalf("movie duration = %g, want 1 second", duration)
	}
	pcm, err := exec.Command(ffmpeg, "-v", "error", "-i", path, "-map", "0:a:0", "-f", "s16le", "pipe:1").Output()
	if err != nil {
		t.Fatal(err)
	}
	nonzero := false
	for _, b := range pcm {
		nonzero = nonzero || b != 0
	}
	if !nonzero {
		t.Fatal("MP4 lost the application's directly mixed sound")
	}
	image, err := exec.Command(ffmpeg, "-v", "error", "-i", path, "-map", "0:v:0", "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgba", "pipe:1").Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(image) != 32*32*4 {
		t.Fatalf("decoded window has %d RGBA bytes", len(image))
	}
	center := (16*32 + 16) * 4
	if image[0] > 8 || image[1] > 8 || image[2] > 8 || image[center] < 190 || image[center+1] > 90 {
		t.Fatal("video did not preserve the scene with black letterboxing")
	}
}

func TestRecorderNeverOverwritesOutput(t *testing.T) {
	ffmpeg := ffmpegForTest(t)
	path := filepath.Join(t.TempDir(), "keep.mp4")
	config := Config{Path: path, Width: 8, Height: 4, OutputWidth: 32, OutputHeight: 32, FFmpegPath: ffmpeg}
	r, err := New(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Abort()
	if err := r.WriteFrame(frameRGBA(8, 4), nil, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("user-owned movie"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); !errors.Is(err, os.ErrExist) {
		t.Fatalf("Close with newly created output = %v, want exists error", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "user-owned movie" {
		t.Fatalf("existing output changed: %q, %v", data, err)
	}
	if _, err := New(config, nil); !errors.Is(err, os.ErrExist) {
		t.Fatalf("New with existing output = %v, want exists error", err)
	}
	if _, err := os.Stat(r.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed publication left a temporary directory: %v", err)
	}
}

func TestRecorderInvalidFrameCleansUpOnClose(t *testing.T) {
	ffmpeg := ffmpegForTest(t)
	path := filepath.Join(t.TempDir(), "invalid.mp4")
	r, err := New(Config{Path: path, Width: 8, Height: 4, FFmpegPath: ffmpeg}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Abort()
	if err := r.WriteFrame([]byte{0}, nil, 0, 0); err == nil {
		t.Fatal("incomplete RGBA frame was accepted")
	}
	if err := r.Close(); err == nil {
		t.Fatal("Close after invalid frame should report the original error")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid recording published an output: %v", err)
	}
	if _, err := os.Stat(r.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid recording left a temporary directory: %v", err)
	}
}

func TestRecorderAbortRemovesOnlyTemporaryFiles(t *testing.T) {
	ffmpeg := ffmpegForTest(t)
	path := filepath.Join(t.TempDir(), "aborted.mp4")
	r, err := New(Config{Path: path, Width: 8, Height: 4, FFmpegPath: ffmpeg}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Abort()
	r.Abort()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("aborted output exists: %v", err)
	}
	if _, err := os.Stat(r.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("aborted temporary directory exists: %v", err)
	}
	if err := r.Close(); err == nil {
		t.Fatal("Close after Abort should report cancellation")
	}
}

func TestEncoderErrorOutputIsBounded(t *testing.T) {
	b := &tailBuffer{}
	_, _ = b.Write([]byte(strings.Repeat("a", 40000)))
	_, _ = b.Write([]byte("last error"))
	if len(b.data) != 32*1024 || !strings.HasSuffix(b.message(), "last error") {
		t.Fatal("FFmpeg stderr tail is not bounded or lost its final diagnostic")
	}
}
