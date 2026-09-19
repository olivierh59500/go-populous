// Package recording exports simulation frames and directly mixed game audio to
// an MP4. It does not open a microphone, system capture device, or audio output.
package recording

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"go-populous/internal/populous"
)

// Config describes packed RGBA source frames and the exported window size.
// OutputWidth and OutputHeight default to 960x720; source dimensions are required.
type Config struct {
	Path                      string
	Width, Height             int
	OutputWidth, OutputHeight int
	FFmpegPath                string
}

// Recorder synchronously consumes one image per simulation tick. It holds only
// one audio frame and a bounded set of sound voices in memory. Its methods must
// all be called from the same goroutine.
type Recorder struct {
	config  Config
	dir     string
	ffmpeg  string
	encoder *exec.Cmd
	stdin   io.WriteCloser
	stderr  *tailBuffer
	audio   *os.File
	mixer   *mixer
	frames  int64
	closed  bool
	err     error
}

func New(config Config, bank *populous.SoundBank) (*Recorder, error) {
	if config.Path == "" {
		return nil, errors.New("recording: an output path is required")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 16384 || config.Height > 16384 {
		return nil, errors.New("recording: source dimensions must be between 1 and 16384")
	}
	if config.OutputWidth == 0 {
		config.OutputWidth = 960
	}
	if config.OutputHeight == 0 {
		config.OutputHeight = 720
	}
	if config.OutputWidth <= 0 || config.OutputHeight <= 0 || config.OutputWidth > 16384 || config.OutputHeight > 16384 || config.OutputWidth%2 != 0 || config.OutputHeight%2 != 0 {
		return nil, errors.New("recording: output dimensions must be positive even numbers no larger than 16384")
	}
	path, err := filepath.Abs(config.Path)
	if err != nil {
		return nil, fmt.Errorf("recording output path: %w", err)
	}
	config.Path = path
	if _, err := os.Lstat(path); err == nil {
		return nil, fmt.Errorf("recording output already exists: %w", os.ErrExist)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("recording output: %w", err)
	}
	ffmpeg := config.FFmpegPath
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	ffmpeg, err = exec.LookPath(ffmpeg)
	if err != nil {
		return nil, fmt.Errorf("recording requires FFmpeg: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("recording output directory: %w", err)
	}
	dir, err := os.MkdirTemp(filepath.Dir(path), ".populous-recording-*")
	if err != nil {
		return nil, fmt.Errorf("recording temporary directory: %w", err)
	}
	r := &Recorder{config: config, dir: dir, ffmpeg: ffmpeg, stderr: &tailBuffer{}, mixer: newMixer(bank)}
	r.audio, err = os.Create(filepath.Join(dir, "audio.pcm"))
	if err != nil {
		r.Abort()
		return nil, fmt.Errorf("recording audio: %w", err)
	}
	filter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2:flags=neighbor,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1,fps=30", config.OutputWidth, config.OutputHeight, config.OutputWidth, config.OutputHeight)
	r.encoder = exec.Command(ffmpeg,
		"-hide_banner", "-loglevel", "error", "-nostdin", "-n",
		"-f", "rawvideo", "-pixel_format", "rgba", "-video_size", fmt.Sprintf("%dx%d", config.Width, config.Height),
		"-framerate", strconv.Itoa(FramesPerSecond), "-i", "pipe:0", "-an",
		"-vf", filter, "-c:v", "libx264", "-preset", "veryfast", "-crf", "18", "-pix_fmt", "yuv420p",
		filepath.Join(dir, "video.mp4"),
	)
	r.encoder.Stderr = r.stderr
	r.stdin, err = r.encoder.StdinPipe()
	if err == nil {
		err = r.encoder.Start()
	}
	if err != nil {
		r.Abort()
		return nil, fmt.Errorf("starting recording encoder: %w", err)
	}
	return r, nil
}

// WriteFrame records 1/8 second. Events start at the beginning of that tick;
// music and the population heartbeat run at their original 20 ms cadence.
// rgba is consumed before this call returns, so its storage may be reused.
func (r *Recorder) WriteFrame(rgba []byte, events []int, playerPop, opponentPop int) error {
	if r.closed {
		return errors.New("recording is closed")
	}
	if r.err != nil {
		return r.err
	}
	want := r.config.Width * r.config.Height * 4
	if len(rgba) != want {
		r.err = fmt.Errorf("recording frame contains %d RGBA bytes, want %d", len(rgba), want)
		return r.err
	}
	if err := writeAll(r.stdin, rgba); err != nil {
		r.err = fmt.Errorf("recording video: %w%s", err, r.stderr.message())
		return r.err
	}
	if err := writeAll(r.audio, r.mixer.frame(events, playerPop, opponentPop)); err != nil {
		r.err = fmt.Errorf("recording audio: %w", err)
		return r.err
	}
	r.frames++
	return nil
}

// Close finishes encoding and atomically publishes the MP4. It never replaces
// an existing output, including a file created while recording was underway.
func (r *Recorder) Close() error {
	if r.closed {
		return r.err
	}
	if r.err != nil {
		r.Abort()
		return r.err
	}
	if r.frames == 0 {
		r.err = errors.New("recording contains no frames")
		r.Abort()
		return r.err
	}
	r.closed = true
	defer r.cleanup()
	pipeErr := r.stdin.Close()
	encodeErr := r.encoder.Wait()
	audioErr := r.audio.Close()
	if encodeErr != nil {
		r.err = fmt.Errorf("encoding recording video: %w%s", encodeErr, r.stderr.message())
		return r.err
	}
	if err := errors.Join(pipeErr, audioErr); err != nil {
		r.err = fmt.Errorf("finishing recording streams: %w", err)
		return r.err
	}
	// The PCM has the exact simulation duration; 30 fps video is quantized to
	// the nearest output frame. Limit the muxer to the intended match duration.
	duration := fmt.Sprintf("%.3f", float64(r.frames)/FramesPerSecond)
	mux := exec.Command(r.ffmpeg,
		"-hide_banner", "-loglevel", "error", "-nostdin", "-n",
		"-i", filepath.Join(r.dir, "video.mp4"),
		"-f", "s16le", "-ar", strconv.Itoa(SampleRate), "-ac", "2", "-i", filepath.Join(r.dir, "audio.pcm"),
		"-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-c:a", "aac", "-b:a", "192k",
		"-t", duration, "-movflags", "+faststart", filepath.Join(r.dir, "complete.mp4"),
	)
	stderr := &tailBuffer{}
	mux.Stderr = stderr
	if err := mux.Run(); err != nil {
		r.err = fmt.Errorf("muxing recording audio: %w%s", err, stderr.message())
		return r.err
	}
	// A same-filesystem hard link is atomic and fails if the target exists.
	// Unlike Rename on Unix, this cannot accidentally overwrite a user's file.
	if err := os.Link(filepath.Join(r.dir, "complete.mp4"), r.config.Path); err != nil {
		r.err = fmt.Errorf("publishing recording without overwriting %q: %w", r.config.Path, err)
	}
	return r.err
}

// Abort stops FFmpeg and removes only this recorder's temporary files. A
// published MP4 or a pre-existing output file is never removed.
func (r *Recorder) Abort() {
	if r == nil || r.closed {
		return
	}
	r.closed = true
	if r.err == nil {
		r.err = errors.New("recording aborted")
	}
	if r.stdin != nil {
		_ = r.stdin.Close()
	}
	if r.encoder != nil && r.encoder.Process != nil {
		_ = r.encoder.Process.Kill()
		_ = r.encoder.Wait()
	}
	if r.audio != nil {
		_ = r.audio.Close()
	}
	r.cleanup()
}

func (r *Recorder) cleanup() {
	for _, name := range []string{"audio.pcm", "video.mp4", "complete.mp4"} {
		_ = os.Remove(filepath.Join(r.dir, name))
	}
	_ = os.Remove(r.dir)
}

func writeAll(w io.Writer, bytes []byte) error {
	for len(bytes) > 0 {
		n, err := w.Write(bytes)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		bytes = bytes[n:]
	}
	return nil
}

// FFmpeg writes asynchronously, and a pipe failure can occur before Wait.
type tailBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	const limit = 32 * 1024
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if len(p) >= limit {
		b.data = append(b.data[:0], p[len(p)-limit:]...)
	} else {
		if overflow := len(b.data) + len(p) - limit; overflow > 0 {
			copy(b.data, b.data[overflow:])
			b.data = b.data[:len(b.data)-overflow]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}

func (b *tailBuffer) message() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if text := strings.TrimSpace(string(b.data)); text != "" {
		return ": " + text
	}
	return ""
}
