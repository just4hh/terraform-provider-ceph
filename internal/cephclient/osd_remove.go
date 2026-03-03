package cephclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ceph/go-ceph/rados"
)

type safeToDestroyResponse struct {
	SafeToDestroy []int `json:"safe_to_destroy"`
}

// MarkOSDOut marks the OSD out of the cluster.
func (c *Client) MarkOSDOut(ctx context.Context, osdID int) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix": "osd out",
		"id":     osdID,
		"format": "json",
	}

	_, _, err := c.monCommandCtx(ctx, cmd)
	if err != nil {
		// If already out or gone, treat as success.
		if errors.Is(err, rados.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("osd out %d: %w", osdID, err)
	}

	return nil
}

// isSafeToDestroy checks if Ceph reports the OSD as safe to destroy.
func (c *Client) isSafeToDestroy(ctx context.Context, osdID int) (bool, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix": "osd safe-to-destroy",
		"ids":    fmt.Sprintf("%d", osdID),
		"format": "json",
	}

	buf, _, err := c.monCommandCtx(ctx, cmd)
	if err != nil {
		if errors.Is(err, rados.ErrNotFound) {
			return true, nil
		}
		return false, fmt.Errorf("osd safe-to-destroy %d: %w", osdID, err)
	}

	var resp safeToDestroyResponse
	if err := json.Unmarshal(buf, &resp); err != nil {
		return false, fmt.Errorf("unmarshal safe-to-destroy: %w", err)
	}

	for _, id := range resp.SafeToDestroy {
		if id == osdID {
			return true, nil
		}
	}

	return false, nil
}

// purgeOSD removes the OSD, its CRUSH entry, and its auth key.
func (c *Client) purgeOSD(ctx context.Context, osdID int) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix":               "osd purge",
		"id":                   osdID,
		"yes_i_really_mean_it": true,
		"format":               "json",
	}

	_, _, err := c.monCommandCtx(ctx, cmd)
	if err != nil {
		if errors.Is(err, rados.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("osd purge %d: %w", osdID, err)
	}

	return nil
}

// DeleteOSDSafely performs:
//  1. osd out
//  2. wait until safe-to-destroy
//  3. osd purge
func (c *Client) DeleteOSDSafely(ctx context.Context, osdID int, wait time.Duration) error {
	// Step 1: mark out
	if err := c.MarkOSDOut(ctx, osdID); err != nil {
		return err
	}

	// Step 2: wait for safe-to-destroy
	deadline := time.Now().Add(wait)
	for {
		safe, err := c.isSafeToDestroy(ctx, osdID)
		if err != nil {
			return err
		}
		if safe {
			break
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for OSD %d to become safe-to-destroy", osdID)
		}

		time.Sleep(10 * time.Second)
	}

	// Step 3: purge
	return c.purgeOSD(ctx, osdID)
}
