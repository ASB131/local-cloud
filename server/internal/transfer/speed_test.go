package transfer

import (
	"bytes"
	"io"
	"testing"
	"time"
)

func intPtr(v int) *int { return &v }

func TestEffectiveLimitKbps(t *testing.T) {
	t.Parallel()
	global := intPtr(5000)
	user := intPtr(1000)
	if got := EffectiveLimitKbps(SpeedLimits{UserKbps: user, GlobalKbps: global}); got == nil || *got != 1000 {
		t.Fatalf("expected user cap 1000, got %v", got)
	}
	userZero := intPtr(0)
	if got := EffectiveLimitKbps(SpeedLimits{UserKbps: userZero, GlobalKbps: global}); got != nil {
		t.Fatalf("user 0 = unlimited, got %v", got)
	}
	if got := EffectiveLimitKbps(SpeedLimits{GlobalKbps: global}); got == nil || *got != 5000 {
		t.Fatalf("expected global 5000, got %v", got)
	}
	userHigh := intPtr(8000)
	if got := EffectiveLimitKbps(SpeedLimits{UserKbps: userHigh, GlobalKbps: global}); got == nil || *got != 5000 {
		t.Fatalf("expected min(global,user)=5000, got %v", got)
	}
}

func TestThrottleReaderEnforcesMinimumDuration(t *testing.T) {
	const kbps = 40
	payload := bytes.Repeat([]byte("x"), 4000)
	start := time.Now()
	_, err := io.Copy(io.Discard, ThrottleReader(bytes.NewReader(payload), kbps))
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if elapsed < 80*time.Millisecond {
		t.Fatalf("expected throttling >= 80ms, got %v", elapsed)
	}
}
