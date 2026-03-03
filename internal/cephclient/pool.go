package cephclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ceph/go-ceph/rados"
)

type PoolInfo struct {
	Name                string         `json:"pool_name"`
	PGNum               uint64         `json:"pg_num"`
	Size                uint64         `json:"size"`
	MinSize             uint64         `json:"min_size"`
	AutoscaleMode       string         `json:"pg_autoscale_mode"`
	ApplicationMetadata map[string]any `json:"application_metadata"`
}

func (c *Client) CreatePool(ctx context.Context, name string, pgNum uint64, size uint64, minSize uint64, application string, autoscaleMode string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	// Create pool
	cmd := map[string]any{
		"prefix":  "osd pool create",
		"pool":    name,
		"pg_num":  pgNum,
		"pgp_num": pgNum,
		"format":  "json",
	}
	if _, _, err := c.monCommandCtx(ctx, cmd); err != nil {
		return fmt.Errorf("osd pool create: %w", err)
	}

	// Set size
	if size > 0 {
		if err := c.SetPoolProperty(ctx, name, "size", fmt.Sprintf("%d", size)); err != nil {
			return fmt.Errorf("set size: %w", err)
		}
	}

	// Set min_size
	if minSize > 0 {
		if err := c.SetPoolProperty(ctx, name, "min_size", fmt.Sprintf("%d", minSize)); err != nil {
			return fmt.Errorf("set min_size: %w", err)
		}
	}

	// Enable application
	if application != "" {
		if err := c.SetPoolApplication(ctx, name, application); err != nil {
			return fmt.Errorf("set application: %w", err)
		}
	}

	// Autoscale mode
	if autoscaleMode != "" {
		if err := c.SetPoolProperty(ctx, name, "pg_autoscale_mode", autoscaleMode); err != nil {
			return fmt.Errorf("set autoscale mode: %w", err)
		}
	}

	return nil
}

func (c *Client) ReadPool(ctx context.Context, name string) (PoolInfo, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix": "osd pool ls",
		"detail": "detail",
		"format": "json",
	}

	buf, _, err := c.monCommandCtx(ctx, cmd)
	if err != nil {
		if errors.Is(err, rados.ErrNotFound) || strings.Contains(err.Error(), "ret=-2") {
			return PoolInfo{}, ErrNotFound
		}
		return PoolInfo{}, fmt.Errorf("osd pool ls detail: %w", err)
	}

	var pools []PoolInfo
	if err := json.Unmarshal(buf, &pools); err != nil {
		return PoolInfo{}, fmt.Errorf("unmarshal pool detail: %w", err)
	}

	for _, p := range pools {
		if p.Name == name {
			return p, nil
		}
	}

	// log.Printf("[DEBUG] ReadPool: searching for pool %q", name)

	// log.Printf("[DEBUG] ReadPool: raw JSON from Ceph:\n%s", string(buf))

	// After unmarshalling into []PoolInfo:
	// for _, p := range pools {
	// 	// log.Printf("[DEBUG] ReadPool: found pool_name=%q pg_num=%d size=%d",
	// 		p.Name, p.PGNum, p.Size)
	// }

	// log.Printf("[DEBUG] ReadPool: pool %q NOT FOUND in %d pools", name, len(pools))

	return PoolInfo{}, ErrNotFound
}

func (c *Client) DeletePool(ctx context.Context, name string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	// 1. Best-effort: disable common applications (rbd, cephfs, rgw, etc.)
	// We ignore errors here because the pool may not have these apps enabled.
	apps := []string{"rbd", "cephfs", "rgw"}
	for _, app := range apps {
		disable := map[string]any{
			"prefix":               "osd pool application disable",
			"pool":                 name,
			"app":                  app,
			"yes_i_really_mean_it": true,
			"format":               "json",
		}
		_, _, _ = c.monCommandCtx(ctx, disable)
	}

	// 2. Delete the pool
	deleteCmd := map[string]any{
		"prefix":                      "osd pool delete",
		"pool":                        name,
		"pool2":                       name,
		"yes_i_really_really_mean_it": true,
		"format":                      "json",
	}

	_, _, err := c.monCommandCtx(ctx, deleteCmd)
	if err == nil {
		return nil
	}

	// Normalize common Ceph errors
	if errors.Is(err, rados.ErrNotFound) || strings.Contains(err.Error(), "ret=-2") {
		return ErrNotFound
	}

	// Ceph often returns: "rados: ret=-1, Operation not permitted"
	if strings.Contains(err.Error(), "ret=-1") &&
		strings.Contains(strings.ToLower(err.Error()), "operation not permitted") {

		return fmt.Errorf(
			"osd pool delete: operation not permitted. "+
				"Ceph requires 'mon_allow_pool_delete = true' on the monitors. "+
				"Set it (e.g. 'ceph config set mon mon_allow_pool_delete true') "+
				"and retry the destroy: %w",
			err,
		)
	}

	return fmt.Errorf("osd pool delete: %w", err)
}

func (c *Client) SetPoolProperty(ctx context.Context, name, prop, value string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix": "osd pool set",
		"pool":   name,
		"var":    prop,
		"val":    value,
		"format": "json",
	}
	if _, _, err := c.monCommandCtx(ctx, cmd); err != nil {
		return fmt.Errorf("osd pool set %s=%s: %w", prop, value, err)
	}
	return nil
}

func (c *Client) SetPoolApplication(ctx context.Context, name, app string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix": "osd pool application enable",
		"pool":   name,
		"app":    app,
		"format": "json",
	}
	if _, _, err := c.monCommandCtx(ctx, cmd); err != nil {
		return fmt.Errorf("osd pool application enable %s: %w", app, err)
	}
	return nil
}
