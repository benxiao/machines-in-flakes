package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// pregenThumbs fills the thumbnail cache in the background after a reindex,
// so grid views don't generate on demand. One pass at a time process-wide;
// interactive requests share thumbSem and compete fairly.
func (a *App) pregenThumbs(paths []string) {
	if !a.pregenBusy.CompareAndSwap(false, true) {
		return
	}
	defer a.pregenBusy.Store(false)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	var made, failed int
	for _, p := range paths {
		if ctx.Err() != nil {
			break
		}
		cacheFile := thumbCachePath(p)
		if _, err := os.Stat(cacheFile); err == nil {
			continue
		}
		acquired := false
		select {
		case thumbSem <- struct{}{}:
			acquired = true
		case <-ctx.Done():
		}
		if !acquired {
			break
		}
		err := a.generateThumb(ctx, p, cacheFile, classifyExt(filepath.Ext(p)))
		<-thumbSem
		if err != nil {
			failed++
		} else {
			made++
		}
		time.Sleep(200 * time.Millisecond)
	}
	if made > 0 || failed > 0 {
		log.Printf("thumb pregen: generated %d, failed %d (of %d candidates)", made, failed, len(paths))
	}
}

const thumbCacheDir = "/var/lib/filebrowser/thumbs"

// A grid view of a large folder can request dozens of thumbnails at once;
// cap concurrent ffmpeg spawns.
var thumbSem = make(chan struct{}, 3)

func thumbCachePath(absPath string) string {
	h := sha256.Sum256([]byte(absPath))
	return filepath.Join(thumbCacheDir, hex.EncodeToString(h[:])+".jpg")
}

func (a *App) generateThumb(ctx context.Context, absPath, cacheFile, fileType string) error {
	tmp := cacheFile + ".tmp"
	var args []string
	switch fileType {
	case "video":
		args = []string{"-y", "-ss", "10", "-i", absPath, "-vframes", "1", "-vf", "scale=320:-2", "-q:v", "5", "-f", "image2", tmp}
	case "photo":
		args = []string{"-y", "-i", absPath, "-vf", "scale=320:-2", "-q:v", "5", "-f", "image2", tmp}
	default:
		return fmt.Errorf("unsupported file type %q", fileType)
	}
	if err := os.MkdirAll(thumbCacheDir, 0755); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, a.ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("%v %s", err, stderr.String())
	}
	return os.Rename(tmp, cacheFile)
}

func serveThumb(w http.ResponseWriter, r *http.Request, cacheFile string) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "max-age=86400")
	http.ServeFile(w, r, cacheFile)
}

func (a *App) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Query().Get("path")
	if rawPath == "" {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Clean(rawPath)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	cacheFile := thumbCachePath(absPath)
	if _, err := os.Stat(cacheFile); err == nil {
		serveThumb(w, r, cacheFile)
		return
	}

	if a.ffmpegPath == "" {
		http.Error(w, "transcoding not configured", http.StatusServiceUnavailable)
		return
	}

	fileType := classifyExt(filepath.Ext(absPath))
	if fileType != "video" && fileType != "photo" {
		http.NotFound(w, r)
		return
	}

	// Archive-internal files extract to the cache; the thumb cache key above
	// stays on the virtual path so it survives cache eviction.
	srcPath, err := a.resolveMediaPath(r.Context(), absPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	select {
	case thumbSem <- struct{}{}:
	case <-r.Context().Done():
		return
	}
	defer func() { <-thumbSem }()

	// Another request may have generated it while we waited on the semaphore.
	if _, err := os.Stat(cacheFile); err != nil {
		if err := a.generateThumb(r.Context(), srcPath, cacheFile, fileType); err != nil {
			log.Printf("thumbnail %s: %v", absPath, err)
			http.Error(w, "thumbnail generation failed", http.StatusInternalServerError)
			return
		}
	}
	serveThumb(w, r, cacheFile)
}

// ---- Transcoding settings ----

type TranscodeSettings struct {
	CRF                int
	Preset             string
	MaxWidth           int
	VideoKbps          int
	AudioKbps          int
	SegmentSec         int
	AudioHLS           bool
	AudioHLSThreshold  int // kbps — files above this bitrate get HLS transcoding
	ForceOriginal      bool
	DefaultVolume      float64
}

var validPresets = map[string]bool{
	"ultrafast": true, "superfast": true, "veryfast": true, "faster": true,
	"fast": true, "medium": true, "slow": true, "slower": true, "veryslow": true,
}

func transcodeParamsFromRequest(r *http.Request) TranscodeSettings {
	q := r.URL.Query()
	s := TranscodeSettings{CRF: 23, Preset: "fast", MaxWidth: 1280, VideoKbps: 3000, AudioKbps: 128, SegmentSec: 6, AudioHLS: true, AudioHLSThreshold: 320}
	if n, err := strconv.Atoi(q.Get("crf")); err == nil && n >= 0 && n <= 51 {
		s.CRF = n
	}
	if v := q.Get("preset"); validPresets[v] {
		s.Preset = v
	}
	if n, err := strconv.Atoi(q.Get("max_width")); err == nil && n >= 0 && n <= 7680 {
		s.MaxWidth = n
	}
	if n, err := strconv.Atoi(q.Get("video_kbps")); err == nil && n > 0 && n <= 40000 {
		s.VideoKbps = n
	}
	if n, err := strconv.Atoi(q.Get("audio_kbps")); err == nil && n > 0 && n <= 1024 {
		s.AudioKbps = n
	}
	if n, err := strconv.Atoi(q.Get("segment_sec")); err == nil && n >= 2 && n <= 60 {
		s.SegmentSec = n
	}
	if v := q.Get("audio_hls"); v != "" {
		s.AudioHLS = v == "1"
	}
	if n, err := strconv.Atoi(q.Get("audio_hls_threshold")); err == nil && n > 0 && n <= 10000 {
		s.AudioHLSThreshold = n
	}
	return s
}

// Cap concurrent segment transcodes so rapid seeking or many parallel
// streams can't spawn unbounded ffmpeg processes; excess requests queue.
var segSem = make(chan struct{}, 6)

func tsQueryParams(ts TranscodeSettings) string {
	audioHLS := "0"
	if ts.AudioHLS {
		audioHLS = "1"
	}
	return fmt.Sprintf("&crf=%d&preset=%s&max_width=%d&video_kbps=%d&audio_kbps=%d&segment_sec=%d&audio_hls=%s&audio_hls_threshold=%d",
		ts.CRF, ts.Preset, ts.MaxWidth, ts.VideoKbps, ts.AudioKbps, ts.SegmentSec, audioHLS, ts.AudioHLSThreshold)
}

func (a *App) handleHLSPlaylist(w http.ResponseWriter, r *http.Request) {
	if a.ffmpegPath == "" {
		http.Error(w, "transcoding not configured", http.StatusServiceUnavailable)
		return
	}
	rawPath := r.URL.Query().Get("path")
	if rawPath == "" {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Clean(rawPath)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	srcPath, err := a.resolveMediaPath(r.Context(), absPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ffprobePath := filepath.Join(filepath.Dir(a.ffmpegPath), "ffprobe")
	ts := transcodeParamsFromRequest(r)
	if classifyExt(filepath.Ext(absPath)) == "audio" {
		// Redirect to direct file if audio HLS is disabled or bitrate is below threshold.
		if !ts.AudioHLS {
			http.Redirect(w, r, "/file?path="+url.QueryEscape(absPath), http.StatusFound)
			return
		}
		brOut, _ := exec.CommandContext(r.Context(), ffprobePath,
			"-v", "quiet", "-show_entries", "stream=bit_rate",
			"-select_streams", "a:0", "-of", "csv=p=0", srcPath,
		).Output()
		bitrate, _ := strconv.ParseInt(strings.TrimSpace(string(brOut)), 10, 64)
		if bitrate > 0 && bitrate < int64(ts.AudioHLSThreshold)*1000 {
			http.Redirect(w, r, "/file?path="+url.QueryEscape(absPath), http.StatusFound)
			return
		}
	}
	out, err := exec.CommandContext(r.Context(), ffprobePath,
		"-v", "quiet", "-show_entries", "format=duration", "-of", "csv=p=0", srcPath,
	).Output()
	if err != nil {
		http.Error(w, "could not probe media", http.StatusInternalServerError)
		return
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || duration <= 0 {
		http.Error(w, "invalid duration", http.StatusInternalServerError)
		return
	}
	segSec := ts.SegmentSec
	encodedPath := url.QueryEscape(absPath)
	tsP := tsQueryParams(ts)
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", segSec)
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n")
	fullSegments := int(duration) / segSec
	lastDur := duration - float64(fullSegments*segSec)
	for i := range fullSegments {
		fmt.Fprintf(&b, "#EXTINF:%d.000,\n/hls/segment?path=%s&n=%d%s\n", segSec, encodedPath, i, tsP)
	}
	if lastDur > 0.05 {
		fmt.Fprintf(&b, "#EXTINF:%.3f,\n/hls/segment?path=%s&n=%d%s\n", lastDur, encodedPath, fullSegments, tsP)
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Write([]byte(b.String()))
}

func (a *App) handleHLSSegment(w http.ResponseWriter, r *http.Request) {
	if a.ffmpegPath == "" {
		http.Error(w, "transcoding not configured", http.StatusServiceUnavailable)
		return
	}
	rawPath := r.URL.Query().Get("path")
	nStr := r.URL.Query().Get("n")
	if rawPath == "" || nStr == "" {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Clean(rawPath)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	n, err := strconv.Atoi(nStr)
	if err != nil || n < 0 {
		http.NotFound(w, r)
		return
	}
	srcPath, err := a.resolveMediaPath(r.Context(), absPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ts := transcodeParamsFromRequest(r)
	startSec := n * ts.SegmentSec

	select {
	case segSem <- struct{}{}:
	case <-r.Context().Done():
		return
	}
	defer func() { <-segSem }()
	w.Header().Set("Content-Type", "video/mp2t")

	if classifyExt(filepath.Ext(absPath)) == "audio" {
		args := []string{
			"-y",
			"-ss", strconv.Itoa(startSec),
			"-i", srcPath,
			"-t", strconv.Itoa(ts.SegmentSec),
			"-vn",
			"-c:a", "aac", "-b:a", fmt.Sprintf("%dk", ts.AudioKbps),
			"-output_ts_offset", strconv.Itoa(startSec),
			"-muxdelay", "0", "-muxpreload", "0",
			"-f", "mpegts", "pipe:1",
		}
		cmd := exec.CommandContext(r.Context(), a.ffmpegPath, args...)
		var stderr bytes.Buffer
		cmd.Stdout = w
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			log.Printf("transcode segment %s/%d: %v\n%s", absPath, n, err, stderr.String())
		}
		return
	}

	useNvenc := a.nvencOK.Load()
	cw := &countingWriter{w: w}
	cmd := exec.CommandContext(r.Context(), a.ffmpegPath, videoSegmentArgs(ts, srcPath, startSec, useNvenc)...)
	var stderr bytes.Buffer
	cmd.Stdout = cw
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// nvenc init failures happen before any output; if nothing was written
		// yet and the client is still there, retry the segment on the CPU.
		if useNvenc && cw.n == 0 && r.Context().Err() == nil {
			es := stderr.String()
			// Hitting the consumer-card concurrent-session limit is transient;
			// anything else (missing driver libs etc.) disables nvenc until restart.
			if !strings.Contains(es, "OpenEncodeSessionEx") && !strings.Contains(es, "out of memory") {
				a.nvencOK.Store(false)
				log.Printf("nvenc disabled after failure")
			}
			log.Printf("nvenc segment %s/%d failed, retrying with libx264: %v\n%s", absPath, n, err, es)
			stderr.Reset()
			cmd = exec.CommandContext(r.Context(), a.ffmpegPath, videoSegmentArgs(ts, srcPath, startSec, false)...)
			cmd.Stdout = cw
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				log.Printf("transcode segment %s/%d: %v\n%s", absPath, n, err, stderr.String())
			}
			return
		}
		log.Printf("transcode segment %s/%d: %v\n%s", absPath, n, err, stderr.String())
	}
}

// nvenc presets run p1 (fastest) … p7 (slowest/best quality).
var nvencPresetMap = map[string]string{
	"ultrafast": "p1", "superfast": "p2", "veryfast": "p3", "faster": "p4",
	"fast": "p4", "medium": "p5", "slow": "p6", "slower": "p7", "veryslow": "p7",
}

func videoEncoderArgs(ts TranscodeSettings, useNvenc bool) []string {
	if useNvenc {
		preset := nvencPresetMap[ts.Preset]
		if preset == "" {
			preset = "p4"
		}
		// CQ uses the same 0-51 scale as x264 CRF and tracks it closely
		// under the bitrate caps applied below.
		return []string{
			"-c:v", "h264_nvenc",
			"-rc", "vbr", "-cq", strconv.Itoa(ts.CRF),
			"-preset", preset,
			"-profile:v", "main", "-level", "4.0",
			"-pix_fmt", "yuv420p",
			"-bf", "0",
			"-spatial-aq", "1",
		}
	}
	return []string{
		"-c:v", "libx264",
		"-crf", strconv.Itoa(ts.CRF),
		"-preset", ts.Preset,
		"-profile:v", "main", "-level", "4.0",
		"-pix_fmt", "yuv420p",
		"-bf", "0",
	}
}

func videoSegmentArgs(ts TranscodeSettings, absPath string, startSec int, useNvenc bool) []string {
	vbr := fmt.Sprintf("%dk", ts.VideoKbps)
	args := []string{
		"-y",
		"-ss", strconv.Itoa(startSec),
		"-i", absPath,
		"-t", strconv.Itoa(ts.SegmentSec),
	}
	args = append(args, videoEncoderArgs(ts, useNvenc)...)
	// Decode and scale stay on the CPU; the GPU is encode-only. scale_cuda
	// would need hwupload plumbing and complicates the -ss input seek.
	if ts.MaxWidth > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale='min(%d,iw)':-2", ts.MaxWidth))
	}
	args = append(args,
		"-b:v", vbr, "-maxrate", vbr, "-bufsize", fmt.Sprintf("%dk", ts.VideoKbps*2),
		"-c:a", "aac", "-b:a", fmt.Sprintf("%dk", ts.AudioKbps),
		"-output_ts_offset", strconv.Itoa(startSec),
		"-muxdelay", "0", "-muxpreload", "0",
		"-f", "mpegts", "pipe:1",
	)
	return args
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	return n, err
}

// detectNVENC probes whether ffmpeg can open an h264_nvenc encode session
// (needs libnvidia-encode/libcuda visible at runtime).
func detectNVENC(ffmpegPath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-hide_banner", "-f", "lavfi", "-i", "color=black:s=256x256:d=0.2",
		"-c:v", "h264_nvenc", "-f", "null", "-")
	return cmd.Run() == nil
}

func (a *App) handleTranscodeStream(w http.ResponseWriter, r *http.Request) {
	if a.ffmpegPath == "" {
		http.Error(w, "transcoding not configured", http.StatusServiceUnavailable)
		return
	}
	rawPath := r.URL.Query().Get("path")
	if rawPath == "" {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Clean(rawPath)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	srcPath, err := a.resolveMediaPath(r.Context(), absPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info, err := os.Stat(srcPath); err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	audioKbps := 128
	if n, err := strconv.Atoi(r.URL.Query().Get("audio_kbps")); err == nil && n > 0 && n <= 1024 {
		audioKbps = n
	}
	// Server-side time-stretch (pitch preserved): rubberband beats the
	// browser's real-time WSOLA stretcher, which warbles on sustained tones.
	speed := 1.0
	if s, err := strconv.ParseFloat(r.URL.Query().Get("speed"), 64); err == nil {
		speed = math.Min(2.0, math.Max(0.5, s))
	}
	if math.Abs(speed-1) < 0.01 {
		speed = 1
	}
	// Start offset in source seconds, so a mid-track speed change only
	// re-encodes the remainder of the track instead of the whole file.
	startSec := 0.0
	if s, err := strconv.ParseFloat(r.URL.Query().Get("ss"), 64); err == nil && s > 0 {
		startSec = s
	}
	// Sources that are already AAC (m4a/aac) remux losslessly to ADTS instead
	// of re-encoding, so this endpoint is safe as the universal audio path.
	codec := ""
	ffprobePath := filepath.Join(filepath.Dir(a.ffmpegPath), "ffprobe")
	if out, err := exec.CommandContext(r.Context(), ffprobePath,
		"-v", "error", "-select_streams", "a:0",
		"-show_entries", "stream=codec_name", "-of", "csv=p=0", srcPath).Output(); err == nil {
		codec = strings.TrimSpace(string(out))
	}
	var args []string
	args = append(args, "-y")
	if startSec > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", startSec))
	}
	args = append(args, "-i", srcPath, "-vn")
	if codec == "aac" && speed == 1 {
		args = append(args, "-c:a", "copy")
	} else {
		if speed != 1 {
			args = append(args, "-af", fmt.Sprintf("rubberband=tempo=%.4f", speed))
		}
		args = append(args, "-c:a", "aac", "-b:a", fmt.Sprintf("%dk", audioKbps))
	}
	args = append(args, "-f", "adts", "pipe:1")
	cmd := exec.CommandContext(r.Context(), a.ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stdout = w
	cmd.Stderr = &stderr
	w.Header().Set("Content-Type", "audio/aac")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if err := cmd.Run(); err != nil {
		log.Printf("transcode stream %s: %v\n%s", absPath, err, stderr.String())
	}
}
