//go:build windows && cgo

package main

import (
	"context"
	"errors"
	"image"
	"testing"
	"time"
)

func TestExtensionDetailCaptureWaitsForGPUFrameAndActualSize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var frames, calls int
	pixels, proof, err := waitExtensionDetailGPUCapture(ctx, 3, func() (uint64, error) {
		frames++
		return uint64(frames), nil
	}, func() (int, int, error) { return 90, 76, nil }, func() (*image.RGBA, error) {
		calls++
		if frames < 3 {
			t.Fatal("captured pixels before the real completion floor")
		}
		switch calls {
		case 1:
			return nil, errors.New("GPU readback has not caught up with window resize")
		case 2:
			return image.NewRGBA(image.Rect(0, 0, 128, 82)), nil
		default:
			return image.NewRGBA(image.Rect(0, 0, 90, 76)), nil
		}
	})
	if err != nil || pixels.Bounds().Dx() != 90 || proof.Completed != 5 || proof.ResizeRetries != 2 || calls != 3 {
		t.Fatal("capture accepted stale frame/size", pixels, proof, err, calls)
	}
}

func TestExtensionDetailCaptureFailsUnknownErrorsAndHonorsDeadline(t *testing.T) {
	unknown := errors.New("GPU readback identity/frame is not ready")
	var calls int
	_, _, err := waitExtensionDetailGPUCapture(context.Background(), 3, func() (uint64, error) { return 3, nil }, func() (int, int, error) { return 90, 76, nil }, func() (*image.RGBA, error) { calls++; return nil, unknown })
	if !errors.Is(err, unknown) || calls != 1 {
		t.Fatal("unknown capture error was swallowed", err, calls)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 65*time.Millisecond)
	defer cancel()
	_, proof, err := waitExtensionDetailGPUCapture(ctx, 3, func() (uint64, error) { return 2, nil }, func() (int, int, error) { t.Fatal("size requested for old frame"); return 0, 0, nil }, func() (*image.RGBA, error) { t.Fatal("old frame captured"); return nil, nil })
	if !errors.Is(err, context.DeadlineExceeded) || proof.Completed != 2 {
		t.Fatal("existing deadline not preserved", proof, err)
	}
}
