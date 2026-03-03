package cephclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type OSDInfo struct {
	ID              int     `json:"osd"`
	Up              int     `json:"up"`
	In              int     `json:"in"`
	Weight          float64 `json:"weight"`
	PrimaryAffinity float64 `json:"primary_affinity,omitempty"`
	// Host and other placement info usually come from osd tree,
	// but some deployments include it here as well.
	// We keep it optional/computed on the Terraform side.
}

type osdDump struct {
	OSDs []OSDInfo `json:"osds"`
}

// ListOSDs returns all OSDs from `ceph osd dump`.
func (c *Client) ListOSDs(ctx context.Context) ([]OSDInfo, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix": "osd dump",
		"format": "json",
	}

	buf, _, err := c.monCommandCtx(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("osd dump: %w", err)
	}

	var dump osdDump
	if err := json.Unmarshal(buf, &dump); err != nil {
		return nil, fmt.Errorf("unmarshal osd dump: %w", err)
	}

	return dump.OSDs, nil
}

// GetOSD returns a single OSD by ID.
func (c *Client) GetOSD(ctx context.Context, id int) (OSDInfo, error) {
	osds, err := c.ListOSDs(ctx)
	if err != nil {
		return OSDInfo{}, err
	}

	for _, o := range osds {
		if o.ID == id {
			return o, nil
		}
	}

	return OSDInfo{}, ErrNotFound
}

// SetOSDInOut marks an OSD in or out.
func (c *Client) SetOSDInOut(ctx context.Context, id int, in bool) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	prefix := "osd out"
	if in {
		prefix = "osd in"
	}

	cmd := map[string]any{
		"prefix": prefix,
		"id":     id, // <-- FIXED
		"format": "json",
	}

	_, _, err := c.monCommandCtx(ctx, cmd)
	if err != nil {
		if errors.Is(err, ErrNotFound) ||
			strings.Contains(err.Error(), "ret=-2") {
			return ErrNotFound
		}
		return fmt.Errorf("%s %d: %w", prefix, id, err)
	}

	return nil
}

// SetOSDWeight sets the CRUSH weight for an OSD.
func (c *Client) SetOSDWeight(ctx context.Context, id int, weight float64) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix": "osd reweight",
		"id":     id,
		"weight": weight,
		"format": "json",
	}

	_, _, err := c.monCommandCtx(ctx, cmd)
	if err != nil {
		if errors.Is(err, ErrNotFound) ||
			strings.Contains(err.Error(), "ret=-2") {
			return ErrNotFound
		}
		return fmt.Errorf("osd reweight %d %f: %w", id, weight, err)
	}

	return nil
}
