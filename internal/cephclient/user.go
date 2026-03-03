package cephclient

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ceph/go-ceph/rados"
)

type UserInfo struct {
	Name string
	Key  string
	Caps map[string]string
}

// parseCaps normalizes caps returned by Ceph into map[string]string.
//
// Supported input shapes:
// 1) map[string]string-like: {"mon":"allow r","osd":"allow rwx pool=rbd"}
// 2) list-of-pairs: [["mon","allow r"],["osd","allow rwx pool=rbd"]]
// 3) list-of-maps: [{"mon":"allow r"},{"osd":"allow rwx pool=rbd"}]
// 4) flat alternating string list: ["mon","allow r","osd","allow rwx pool=rbd"]
func parseCaps(raw any) map[string]string {
	out := map[string]string{}

	// Case 1: map[string]any
	if m, ok := raw.(map[string]any); ok {
		for k, v := range m {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
		return out
	}

	// Case 2/3/4: list forms
	if arr, ok := raw.([]any); ok {
		// Detect flat alternating string list: ["mon","allow r","osd","allow ..."]
		allStrings := true
		for _, it := range arr {
			if _, ok := it.(string); !ok {
				allStrings = false
				break
			}
		}
		if allStrings && len(arr)%2 == 0 {
			for i := 0; i < len(arr); i += 2 {
				sub, _ := arr[i].(string)
				val, _ := arr[i+1].(string)
				if sub != "" && val != "" {
					out[sub] = val
				}
			}
			return out
		}

		// Existing handling: nested pairs or single-key maps
		for _, item := range arr {
			// nested pair: ["mon","allow r"]
			if pair, ok := item.([]any); ok && len(pair) == 2 {
				sub, _ := pair[0].(string)
				val, _ := pair[1].(string)
				if sub != "" && val != "" {
					out[sub] = val
				}
				continue
			}

			// single-key map: {"mon":"allow r"}
			if m, ok := item.(map[string]any); ok {
				for k, v := range m {
					if s, ok := v.(string); ok {
						out[k] = s
					}
				}
				continue
			}
		}
		return out
	}

	return out
}

func parseUserResult(buf []byte) (map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(buf, &arr); err == nil && len(arr) > 0 {
		return arr[0], nil
	}

	var obj map[string]any
	if err := json.Unmarshal(buf, &obj); err == nil && len(obj) > 0 {
		if dump, ok := obj["auth_dump"].([]any); ok && len(dump) > 0 {
			if first, ok := dump[0].(map[string]any); ok {
				return first, nil
			}
		}
		return obj, nil
	}

	return nil, fmt.Errorf("unrecognized auth response format")
}

func (c *Client) CreateUser(ctx context.Context, name string, caps map[string]string) (UserInfo, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix": "auth get-or-create-key",
		"entity": name,
		"caps":   capsToFlatList(caps),
		"format": "json",
	}

	//TEMP DEBUG
	payload, _ := json.Marshal(cmd)
	fmt.Fprintf(os.Stderr, "\n=== AUTH CREATE CMD ===\n%s\n\n", payload)

	buf, _, err := c.monCommandCtx(ctx, cmd)
	if err != nil {
		return UserInfo{}, fmt.Errorf("auth get-or-create-key: %w", err)
	}

	raw, err := parseUserResult(buf)
	if err != nil {
		return UserInfo{}, err
	}

	info := UserInfo{
		Name: name,
		Caps: map[string]string{},
	}

	if key, ok := raw["key"].(string); ok {
		info.Key = key
	}

	if capsRaw, ok := raw["caps"]; ok {
		info.Caps = parseCaps(capsRaw)
	}

	return info, nil
}

func (c *Client) ReadUser(ctx context.Context, name string) (UserInfo, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix": "auth get",
		"entity": name,
		"format": "json",
	}

	// payload := mustJSON(cmd)

	// fmt.Fprintf(os.Stderr, "\n=== auth get payload ===\n%s\n\n", string(payload))

	buf, _, err := c.monCommandCtx(ctx, cmd)
	if err != nil {
		if err == rados.ErrNotFound {
			return UserInfo{}, rados.ErrNotFound
		}
		return UserInfo{}, fmt.Errorf("auth get: %w", err)
	}

	// fmt.Fprintf(os.Stderr, "\n=== auth get response ===\n%s\n\n", string(buf))

	raw, err := parseUserResult(buf)
	if err != nil {
		return UserInfo{}, err
	}

	if _, ok := raw["caps"]; !ok {
		time.Sleep(250 * time.Millisecond)
		buf2, _, err2 := c.monCommandCtx(ctx, cmd)
		if err2 == nil {
			// fmt.Fprintf(os.Stderr, "\n=== auth get response retry ===\n%s\n\n", string(buf2))
			if raw2, err3 := parseUserResult(buf2); err3 == nil {
				raw = raw2
			}
		}
	}

	info := UserInfo{
		Name: name,
		Caps: map[string]string{},
	}

	if key, ok := raw["key"].(string); ok {
		info.Key = key
	}

	if capsRaw, ok := raw["caps"]; ok {
		info.Caps = parseCaps(capsRaw)
	}

	return info, nil
}

func (c *Client) RotateUserKey(ctx context.Context, name string) (UserInfo, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix": "auth rotate",
		"entity": name,
		"format": "json",
	}

	fmt.Println("[DEBUG] cephclient: rotating cephx key for", name)

	buf, _, err := c.monCommandCtx(ctx, cmd)
	if err != nil {
		return UserInfo{}, fmt.Errorf("auth rotate: %w", err)
	}

	fmt.Println("[DEBUG] cephclient: auth rotate response:", string(buf))

	raw, err := parseUserResult(buf)
	if err != nil {
		return UserInfo{}, err
	}

	info := UserInfo{
		Name: name,
		Caps: map[string]string{},
	}

	if key, ok := raw["key"].(string); ok {
		info.Key = key
	}

	if capsRaw, ok := raw["caps"]; ok {
		info.Caps = parseCaps(capsRaw)
	}

	return info, nil
}

func (c *Client) UpdateUser(ctx context.Context, name string, caps map[string]string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix": "auth caps",
		"entity": name,
		"caps":   capsToFlatList(caps),
		"format": "json",
	}

	if _, _, err := c.monCommandCtx(ctx, cmd); err != nil {
		return fmt.Errorf("auth caps: %w", err)
	}

	return nil
}

func (c *Client) DeleteUser(ctx context.Context, name string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	cmd := map[string]any{
		"prefix": "auth del",
		"entity": name,
		"format": "json",
	}

	if _, _, err := c.monCommandCtx(ctx, cmd); err != nil {
		return fmt.Errorf("auth del: %w", err)
	}

	return nil
}

func capsToFlatList(caps map[string]string) []any {
	out := make([]any, 0, len(caps)*2)
	for subsys, val := range caps {
		out = append(out, subsys, val)
	}
	return out
}

func capsToPairList(caps map[string]string) []any {
	out := make([]any, 0, len(caps))
	for subsys, perm := range caps {
		out = append(out, []any{subsys, perm})
	}
	return out
}
