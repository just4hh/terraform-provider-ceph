package cephclient

import (
	"context"
	"encoding/json"
)

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

type monResult struct {
	buf  []byte
	info string
	err  error
}

func (c *Client) monCommandCtx(ctx context.Context, cmd map[string]any) ([]byte, string, error) {
	payload := mustJSON(cmd)

	ch := make(chan monResult, 1)
	go func() {
		buf, info, err := c.conn.MonCommand(payload)
		ch <- monResult{buf: buf, info: info, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, "", ctx.Err()
	case res := <-ch:
		return res.buf, res.info, res.err
	}
}
