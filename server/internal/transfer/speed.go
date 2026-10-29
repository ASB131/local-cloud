package transfer

import (
	"io"
	"time"
)

type SpeedLimits struct {
	UserKbps   *int
	GlobalKbps *int
}

func EffectiveLimitKbps(l SpeedLimits) *int {
	if l.UserKbps != nil && *l.UserKbps == 0 {
		return nil
	}
	var globalPos *int
	if l.GlobalKbps != nil && *l.GlobalKbps > 0 {
		v := *l.GlobalKbps
		globalPos = &v
	}
	if l.UserKbps != nil && *l.UserKbps > 0 {
		u := *l.UserKbps
		if globalPos != nil && u < *globalPos {
			return &u
		}
		if globalPos != nil {
			return globalPos
		}
		return &u
	}
	return globalPos
}

func BytesPerSecond(kbps int) int64 {
	return int64(kbps) * 1000
}

type throttleWriter struct {
	dst         io.Writer
	bytesPerSec int64
}

func ThrottleWriter(dst io.Writer, kbps int) io.Writer {
	if kbps <= 0 {
		return dst
	}
	return &throttleWriter{dst: dst, bytesPerSec: BytesPerSecond(kbps)}
}

func (t *throttleWriter) Write(p []byte) (int, error) {
	start := time.Now()
	n, err := t.dst.Write(p)
	if n > 0 && t.bytesPerSec > 0 {
		expect := time.Duration(int64(n) * int64(time.Second) / t.bytesPerSec)
		if d := time.Since(start); d < expect {
			time.Sleep(expect - d)
		}
	}
	return n, err
}

type throttleReader struct {
	src         io.Reader
	bytesPerSec int64
}

func ThrottleReader(src io.Reader, kbps int) io.Reader {
	if kbps <= 0 {
		return src
	}
	return &throttleReader{src: src, bytesPerSec: BytesPerSecond(kbps)}
}

func (t *throttleReader) Read(p []byte) (int, error) {
	start := time.Now()
	n, err := t.src.Read(p)
	if n > 0 && t.bytesPerSec > 0 {
		expect := time.Duration(int64(n) * int64(time.Second) / t.bytesPerSec)
		if d := time.Since(start); d < expect {
			time.Sleep(expect - d)
		}
	}
	return n, err
}
