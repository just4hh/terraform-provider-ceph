package cephclient

import (
	"context"
	"encoding/json"
	"fmt"
)

// Structures match the output of `ceph osd tree --format=json`
type CrushTree struct {
	Nodes []CrushNode `json:"nodes"`
}

type CrushNode struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	TypeID      int    `json:"type_id"`
	Children    []int  `json:"children"`
	DeviceClass string `json:"device_class,omitempty"`
}

// GetOSDTree returns the CRUSH tree from `ceph osd tree`.
func (c *Client) GetOSDTree(ctx context.Context) (CrushTree, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix": "osd tree",
		"format": "json",
	}

	buf, _, err := c.monCommandCtx(ctx, cmd)
	if err != nil {
		return CrushTree{}, fmt.Errorf("osd tree: %w", err)
	}

	var tree CrushTree
	if err := json.Unmarshal(buf, &tree); err != nil {
		return CrushTree{}, fmt.Errorf("unmarshal osd tree: %w", err)
	}

	return tree, nil
}

// FindOSDHost returns the host name for a given OSD ID.
func (c *Client) FindOSDHost(ctx context.Context, osdID int) (string, error) {
	tree, err := c.GetOSDTree(ctx)
	if err != nil {
		return "", err
	}

	// Build a map for quick lookup
	nodeByID := make(map[int]CrushNode)
	for _, n := range tree.Nodes {
		nodeByID[n.ID] = n
	}

	// Find the OSD node
	osdNode, ok := nodeByID[osdID]
	if !ok {
		return "", fmt.Errorf("OSD %d not found in CRUSH tree", osdID)
	}

	// Walk upward to find a node of type "host"
	for _, parent := range tree.Nodes {
		for _, child := range parent.Children {
			if child == osdNode.ID && parent.Type == "host" {
				return parent.Name, nil
			}
		}
	}

	// Not all clusters use host buckets
	return "", nil
}

// FindOSDDeviceClass returns the device class (hdd, ssd, nvme) for an OSD.
func (c *Client) FindOSDDeviceClass(ctx context.Context, osdID int) (string, error) {
	tree, err := c.GetOSDTree(ctx)
	if err != nil {
		return "", err
	}

	for _, n := range tree.Nodes {
		if n.ID == osdID && n.Type == "osd" {
			return n.DeviceClass, nil
		}
	}

	return "", nil
}

// FindOSDCrushLocation returns a map of CRUSH bucket type -> name for the OSD.
func (c *Client) FindOSDCrushLocation(ctx context.Context, osdID int) (map[string]string, error) {
	tree, err := c.GetOSDTree(ctx)
	if err != nil {
		return nil, err
	}

	// Build lookup map
	nodeByID := make(map[int]CrushNode)
	for _, n := range tree.Nodes {
		nodeByID[n.ID] = n
	}

	// Find the OSD node
	osdNode, ok := nodeByID[osdID]
	if !ok {
		return nil, fmt.Errorf("OSD %d not found in CRUSH tree", osdID)
	}

	// Walk upward: find parents recursively
	location := map[string]string{}

	currentID := osdNode.ID
	for {
		parentFound := false
		for _, n := range tree.Nodes {
			for _, child := range n.Children {
				if child == currentID {
					// This node is the parent
					if n.Type != "osd" {
						location[n.Type] = n.Name
					}
					currentID = n.ID
					parentFound = true
					break
				}
			}
			if parentFound {
				break
			}
		}
		if !parentFound {
			break
		}
	}

	return location, nil
}
