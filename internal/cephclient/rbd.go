package cephclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ceph/go-ceph/rbd"
)

var ErrNotFound = errors.New("rbd image not found")

type RBDImageInfo struct {
	Size uint64
}

func (c *Client) CreateImage(ctx context.Context, pool, name string, size uint64) error {
	ioctx, err := c.conn.OpenIOContext(pool)
	if err != nil {
		return fmt.Errorf("open ioctx: %w", err)
	}
	defer ioctx.Destroy()

	// order = 0 → default object size (4MB)
	_, err = rbd.Create(ioctx, name, size, 0)
	if err != nil {
		return fmt.Errorf("create image: %w", err)
	}

	return nil
}

func (c *Client) GetImage(ctx context.Context, pool, name string) (*RBDImageInfo, error) {
	ioctx, err := c.conn.OpenIOContext(pool)
	if err != nil {
		return nil, fmt.Errorf("open ioctx: %w", err)
	}
	defer ioctx.Destroy()

	img := rbd.GetImage(ioctx, name)
	if err := img.Open(); err != nil {
		if errors.Is(err, rbd.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("open image: %w", err)
	}
	defer img.Close()

	stat, err := img.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}

	return &RBDImageInfo{
		Size: stat.Size,
	}, nil
}

func (c *Client) ResizeImage(ctx context.Context, pool, name string, newSize uint64) error {
	ioctx, err := c.conn.OpenIOContext(pool)
	if err != nil {
		return fmt.Errorf("open ioctx: %w", err)
	}
	defer ioctx.Destroy()

	img := rbd.GetImage(ioctx, name)
	if err := img.Open(); err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer img.Close()

	if err := img.Resize(newSize); err != nil {
		return fmt.Errorf("resize: %w", err)
	}

	return nil
}

func (c *Client) RenameImage(ctx context.Context, pool, oldName, newName string) error {
	ioctx, err := c.conn.OpenIOContext(pool)
	if err != nil {
		return fmt.Errorf("open ioctx: %w", err)
	}
	defer ioctx.Destroy()

	img := rbd.GetImage(ioctx, oldName)
	if err := img.Open(); err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer img.Close()

	if err := img.Rename(newName); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

func (c *Client) DeleteImage(ctx context.Context, pool, name string) error {
	ioctx, err := c.conn.OpenIOContext(pool)
	if err != nil {
		return fmt.Errorf("open ioctx: %w", err)
	}
	defer ioctx.Destroy()

	img := rbd.GetImage(ioctx, name)
	if err := img.Remove(); err != nil {
		if errors.Is(err, rbd.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("remove: %w", err)
	}

	return nil
}

// -----------------------------
// Snapshot Support (metadata‑agnostic)
// -----------------------------

type SnapshotInfo struct {
	CreatedAt time.Time
	Protected bool
}

func (c *Client) CreateSnapshot(ctx context.Context, pool, image, snap string) error {
	ioctx, err := c.conn.OpenIOContext(pool)
	if err != nil {
		return fmt.Errorf("open ioctx: %w", err)
	}
	defer ioctx.Destroy()

	img, err := rbd.OpenImage(ioctx, image, rbd.NoSnapshot)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer img.Close()

	_, err = img.CreateSnapshot(snap)
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}

	return nil
}

func (c *Client) DeleteSnapshot(ctx context.Context, pool, image, snap string) error {
	ioctx, err := c.conn.OpenIOContext(pool)
	if err != nil {
		return fmt.Errorf("open ioctx: %w", err)
	}
	defer ioctx.Destroy()

	img, err := rbd.OpenImage(ioctx, image, rbd.NoSnapshot)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer img.Close()

	s := img.GetSnapshot(snap)
	if err := s.Remove(); err != nil {
		return fmt.Errorf("remove snapshot: %w", err)
	}

	return nil
}

func (c *Client) ProtectSnapshot(ctx context.Context, pool, image, snap string) error {
	ioctx, err := c.conn.OpenIOContext(pool)
	if err != nil {
		return fmt.Errorf("open ioctx: %w", err)
	}
	defer ioctx.Destroy()

	img, err := rbd.OpenImage(ioctx, image, rbd.NoSnapshot)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer img.Close()

	s := img.GetSnapshot(snap)
	if err := s.Protect(); err != nil {
		return fmt.Errorf("protect snapshot: %w", err)
	}

	return nil
}

func (c *Client) UnprotectSnapshot(ctx context.Context, pool, image, snap string) error {
	ioctx, err := c.conn.OpenIOContext(pool)
	if err != nil {
		return fmt.Errorf("open ioctx: %w", err)
	}
	defer ioctx.Destroy()

	img, err := rbd.OpenImage(ioctx, image, rbd.NoSnapshot)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer img.Close()

	s := img.GetSnapshot(snap)
	if err := s.Unprotect(); err != nil {
		return fmt.Errorf("unprotect snapshot: %w", err)
	}

	return nil
}

func (c *Client) SnapshotExists(ctx context.Context, pool, image, snap string) (bool, error) {
	ioctx, err := c.conn.OpenIOContext(pool)
	if err != nil {
		return false, fmt.Errorf("open ioctx: %w", err)
	}
	defer ioctx.Destroy()

	img, err := rbd.OpenImage(ioctx, image, rbd.NoSnapshot)
	if err != nil {
		if errors.Is(err, rbd.ErrNotFound) {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("open image: %w", err)
	}
	defer img.Close()

	snaps, err := img.GetSnapshotNames()
	if err != nil {
		return false, fmt.Errorf("list snapshots: %w", err)
	}

	for _, s := range snaps {
		if s.Name == snap {
			return true, nil
		}
	}

	return false, nil
}

func (c *Client) GetSnapshotInfo(ctx context.Context, pool, image, snap string) (*SnapshotInfo, error) {
	exists, err := c.SnapshotExists(ctx, pool, image, snap)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}

	// Metadata unavailable in this go‑ceph version
	return &SnapshotInfo{}, nil
}
