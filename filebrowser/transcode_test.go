package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTsQueryParams(t *testing.T) {
	ts := TranscodeSettings{CRF: 23, Preset: "fast", MaxWidth: 1280, VideoKbps: 3000, AudioKbps: 128, SegmentSec: 6, AudioHLS: true, AudioHLSThreshold: 320}
	got := tsQueryParams(ts)
	want := "&crf=23&preset=fast&max_width=1280&video_kbps=3000&audio_kbps=128&segment_sec=6&audio_hls=1&audio_hls_threshold=320"
	if got != want {
		t.Errorf("tsQueryParams = %q, want %q", got, want)
	}
	ts.AudioHLS = false
	if !strings.Contains(tsQueryParams(ts), "audio_hls=0") {
		t.Errorf("expected audio_hls=0 when AudioHLS is false")
	}
}

func TestVideoEncoderArgs(t *testing.T) {
	ts := TranscodeSettings{CRF: 23, Preset: "fast"}
	cpu := videoEncoderArgs(ts, false)
	if !containsPair(cpu, "-c:v", "libx264") || !containsPair(cpu, "-crf", "23") {
		t.Errorf("libx264 args missing expected flags: %v", cpu)
	}

	gpu := videoEncoderArgs(ts, true)
	if !containsPair(gpu, "-c:v", "h264_nvenc") || !containsPair(gpu, "-preset", "p4") {
		t.Errorf("nvenc args missing expected mapped preset: %v", gpu)
	}

	unknown := TranscodeSettings{CRF: 20, Preset: "unknown-preset"}
	gpuFallback := videoEncoderArgs(unknown, true)
	if !containsPair(gpuFallback, "-preset", "p4") {
		t.Errorf("expected unmapped x264 preset to fall back to nvenc preset p4, got: %v", gpuFallback)
	}
}

func TestVideoSegmentArgs(t *testing.T) {
	ts := TranscodeSettings{CRF: 23, Preset: "fast", MaxWidth: 1280, VideoKbps: 3000, AudioKbps: 128, SegmentSec: 6}
	args := videoSegmentArgs(ts, "/media/movie.mp4", 30, false)
	if !containsPair(args, "-ss", "30") {
		t.Errorf("expected start offset in args: %v", args)
	}
	if !containsPair(args, "-b:v", "3000k") {
		t.Errorf("expected video bitrate in args: %v", args)
	}
	if !containsArg(args, "-vf") {
		t.Errorf("expected a -vf scale filter when MaxWidth > 0: %v", args)
	}

	ts.MaxWidth = 0
	args = videoSegmentArgs(ts, "/media/movie.mp4", 0, false)
	if containsArg(args, "-vf") {
		t.Errorf("expected no -vf scale filter when MaxWidth is 0: %v", args)
	}
}

func TestTranscodeParamsFromRequest(t *testing.T) {
	r := httptest.NewRequest("GET", "/transcode/stream?crf=18&preset=slow&max_width=1920&video_kbps=6000&audio_kbps=192&segment_sec=10&audio_hls=0&audio_hls_threshold=256", nil)
	ts := transcodeParamsFromRequest(r)
	if ts.CRF != 18 || ts.Preset != "slow" || ts.MaxWidth != 1920 || ts.VideoKbps != 6000 || ts.AudioKbps != 192 || ts.SegmentSec != 10 || ts.AudioHLS || ts.AudioHLSThreshold != 256 {
		t.Errorf("transcodeParamsFromRequest did not honor query params: %+v", ts)
	}

	// Out-of-range / garbage values must fall back to defaults, not error.
	r2 := httptest.NewRequest("GET", "/transcode/stream?crf=999&preset=not-a-real-preset&video_kbps=-5", nil)
	ts2 := transcodeParamsFromRequest(r2)
	if ts2.CRF != 23 || ts2.Preset != "fast" || ts2.VideoKbps != 3000 {
		t.Errorf("expected out-of-range params to fall back to defaults, got: %+v", ts2)
	}
}

func containsPair(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func containsArg(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
