package filewatch

import (
	"context"
	"errors"
	"time"
)

// Replace retries only transient Windows sharing/access failures for at most
// two seconds. verify runs before EVERY attempt, so a delayed save never blindly
// commits over a newly changed disk revision. Permanent errors preserve target.
func Replace(parent context.Context, source, target string, verify func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	delay := 20 * time.Millisecond
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if verify != nil {
			if err := verify(ctx); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		err := renameFile(source, target)
		if err == nil || !transientReplace(err) {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(err, ctx.Err())
		case <-timer.C:
		}
		delay = min(160*time.Millisecond, delay*2)
	}
}
