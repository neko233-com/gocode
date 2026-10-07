//go:build windows && cgo

package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"time"
)

type extensionDetailCaptureProof struct {
	Name             string `json:"name"`
	MinimumCompleted uint64 `json:"minimum_completed"`
	Completed        uint64 `json:"completed"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	ResizeRetries    int    `json:"resize_retries"`
}

// This acceptance helper waits on actual completions and owned-client dimensions.
// The public16 probe reports resize lag as an untyped error; only that exact
// transient can be retried. Identity, malformed readback and other errors fail.
func waitExtensionDetailGPUCapture(ctx context.Context, minimum uint64, completed func() (uint64, error), size func() (int, int, error), capture func() (*image.RGBA, error)) (*image.RGBA, extensionDetailCaptureProof, error) {
	proof := extensionDetailCaptureProof{MinimumCompleted: minimum}
	for {
		if err := ctx.Err(); err != nil {
			return nil, proof, fmt.Errorf("wait for completed native capture: %w", err)
		}
		frame, err := completed()
		if err != nil {
			return nil, proof, err
		}
		proof.Completed = frame
		if frame >= minimum {
			width, height, err := size()
			if err != nil {
				return nil, proof, err
			}
			pixels, err := capture()
			if err != nil && err.Error() != "GPU readback has not caught up with window resize" {
				return nil, proof, err
			}
			if err == nil {
				if pixels == nil {
					return nil, proof, errors.New("native capture returned no pixels")
				}
				afterWidth, afterHeight, err := size()
				if err != nil {
					return nil, proof, err
				}
				if width > 0 && height > 0 && width == afterWidth && height == afterHeight && pixels.Bounds().Dx() == width && pixels.Bounds().Dy() == height {
					proof.Width, proof.Height = width, height
					return pixels, proof, nil
				}
			}
			proof.ResizeRetries++
		}
		timer := time.NewTimer(20 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}
