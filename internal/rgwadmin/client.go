package rgwadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// retryConcurrentModification retries RGW quota operations when RGW returns
// 409 ConcurrentModification. This is an eventual-consistency condition in RGW.
func retryConcurrentModification(ctx context.Context, fn func() error) error {
	backoff := 100 * time.Millisecond
	maxBackoff := 2 * time.Second
	maxAttempts := 8

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := fn()
		if err == nil {
			return nil
		}

		// Detect RGW 409 ConcurrentModification
		msg := err.Error()
		if !strings.Contains(msg, "409") ||
			!strings.Contains(msg, "ConcurrentModification") {
			return err
		}

		lastErr = err

		// Sleep and retry
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	return fmt.Errorf("quota operation failed after retries: %w", lastErr)
}

// Client is a minimal RGW Admin API client.
type Client struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	HTTPClient *http.Client
}

var ErrNotFound = errors.New("not found")

// New creates a new RGW Admin API client.
func New(endpoint, accessKey, secretKey string) *Client {
	return &Client{
		Endpoint:  strings.TrimRight(endpoint, "/"),
		AccessKey: accessKey,
		SecretKey: secretKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an authenticated RGW Admin API request.
func (c *Client) doRequest(
	ctx context.Context,
	method string,
	path string,
	query map[string]string,
	body any,
) ([]byte, error) {
	u, err := url.Parse(c.Endpoint + path)
	if err != nil {
		return nil, fmt.Errorf("invalid RGW endpoint: %w", err)
	}

	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	// Content-Type participates in the SigV2 string-to-sign only when there is a real body.
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	} else {
		req.Header.Del("Content-Type")
	}

	// S3 Signature V2 auth instead of BasicAuth.
	c.signV2(req)
	// log.Printf("RGW REQUEST: %s %s", req.Method, req.URL.String())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("rgw admin error: %s (%d): %s",
			resp.Status, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// -----------------------------------------------------------------------------
// Models
// -----------------------------------------------------------------------------

type UserInfo struct {
	UID         string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Suspended   int    `json:"suspended"`
	Keys        []struct {
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
	} `json:"keys"`
}

type BucketInfo struct {
	Bucket string `json:"bucket"`
	Owner  string `json:"owner"`
	Marker string `json:"marker"`
}

// -----------------------------------------------------------------------------
// User CRUD
// -----------------------------------------------------------------------------

func (c *Client) CreateUser(
	ctx context.Context,
	uid, displayName, email string,
	suspended bool,
) (*UserInfo, error) {
	query := map[string]string{
		"uid":          uid,
		"display-name": displayName,
		"email":        email,
		"generate-key": "false",
		"format":       "json",
	}

	if suspended {
		query["suspended"] = "1"
	}

	var body []byte
	err := retryConcurrentModification(ctx, func() error {
		var e error
		body, e = c.doRequest(ctx, http.MethodPut, "/admin/user", query, nil)
		return e
	})
	if err != nil {
		return nil, err
	}

	var info UserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}

	return &info, nil
}

func (c *Client) ReadUser(ctx context.Context, uid string) (*UserInfo, error) {
	query := map[string]string{
		"uid":    uid,
		"format": "json",
	}

	body, err := c.doRequest(ctx, http.MethodGet, "/admin/user", query, nil)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchUser") ||
			strings.Contains(err.Error(), "404") {
			return nil, ErrNotFound
		}

		return nil, err

	}

	var info UserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}

	return &info, nil
}

// LookupUserByAccessKey finds the RGW user UID that owns the given S3 access key.
func (c *Client) LookupUserByAccessKey(ctx context.Context, accessKey string) (string, error) {
	if accessKey == "" {
		return "", fmt.Errorf("access key is required")
	}

	query := map[string]string{
		"access-key": accessKey,
		"format":     "json",
	}

	body, err := c.doRequest(ctx, http.MethodGet, "/admin/user", query, nil)
	if err != nil {
		return "", err
	}

	var info UserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return "", fmt.Errorf("unmarshal user: %w", err)
	}

	if info.UID == "" {
		return "", fmt.Errorf("no user found for access key %q", accessKey)
	}

	return info.UID, nil
}

func (c *Client) UpdateUser(ctx context.Context, u UserInfo) (*UserInfo, error) {
	query := map[string]string{
		"uid":          u.UID,
		"display-name": u.DisplayName,
		"email":        u.Email,
		"suspended":    fmt.Sprintf("%d", u.Suspended),
		"format":       "json",
	}

	var body []byte
	err := retryConcurrentModification(ctx, func() error {
		var e error
		body, e = c.doRequest(ctx, http.MethodPost, "/admin/user", query, nil)
		return e
	})
	if err != nil {
		return nil, err
	}

	var info UserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}

	return &info, nil
}

func (c *Client) DeleteUser(ctx context.Context, uid string) error {
	query := map[string]string{
		"uid":        uid,
		"purge-data": "true",
		"format":     "json",
	}

	return retryConcurrentModification(ctx, func() error {
		_, err := c.doRequest(ctx, http.MethodDelete, "/admin/user", query, nil)
		return err
	})

}

// -------------------------------
// USER KEY MANAGEMENT
// -------------------------------

type KeyInfo struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

func (c *Client) CreateUserKey(ctx context.Context, uid string) (*KeyInfo, error) {
	// 1. Read keys BEFORE creation
	before, err := c.ListUserKeys(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("pre-create read failed: %w", err)
	}

	// 2. Perform key creation with retry on 409
	query := map[string]string{
		"uid":          uid,
		"key":          "",
		"generate-key": "true",
		"format":       "json",
	}

	err = retryConcurrentModification(ctx, func() error {
		_, err := c.doRequest(ctx, http.MethodPut, "/admin/user", query, nil)
		return err
	})
	if err != nil {
		return nil, err
	}

	// 3. Read keys AFTER creation
	after, err := c.ListUserKeys(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("post-create read failed: %w", err)
	}

	// 4. Diff: find the new key
	beforeSet := map[string]bool{}
	for _, k := range before {
		beforeSet[k.AccessKey] = true
	}

	for _, k := range after {
		if !beforeSet[k.AccessKey] {
			return &KeyInfo{
				AccessKey: k.AccessKey,
				SecretKey: k.SecretKey,
			}, nil
		}
	}

	return nil, fmt.Errorf("could not detect newly created key; RGW returned: %v", after)
}

func (c *Client) DeleteUserKey(ctx context.Context, uid, accessKey string) error {
	if uid == "" || accessKey == "" {
		return fmt.Errorf("uid and accessKey are required")
	}

	query := map[string]string{
		"uid":        uid,
		"key":        "",
		"access-key": accessKey,
		"format":     "json",
	}

	err := retryConcurrentModification(ctx, func() error {
		_, err := c.doRequest(ctx, http.MethodDelete, "/admin/user", query, nil)
		return err
	})
	if err != nil {
		// Treat all "not found" cases as success — required for Terraform idempotency
		if strings.Contains(err.Error(), "NoSuchKey") ||
			strings.Contains(err.Error(), "NoSuchUser") ||
			strings.Contains(err.Error(), "InvalidAccessKeyId") ||
			strings.Contains(err.Error(), "404") {
			return nil
		}
		return err
	}

	return nil

}

// ListUserKeys returns all S3 access keys for the given RGW user.
func (c *Client) ListUserKeys(ctx context.Context, uid string) ([]KeyInfo, error) {
	info, err := c.ReadUser(ctx, uid)
	if err != nil {
		return nil, err
	}

	keys := make([]KeyInfo, 0, len(info.Keys))
	for _, k := range info.Keys {
		keys = append(keys, KeyInfo{
			AccessKey: k.AccessKey,
			SecretKey: k.SecretKey,
		})
	}

	return keys, nil
}

// -----------------------------------------------------------------------------
// Bucket CRUD
// -----------------------------------------------------------------------------

func (c *Client) CreateBucket(ctx context.Context, bucket, owner string) (*BucketInfo, error) {
	query := map[string]string{
		"bucket": bucket,
		"uid":    owner,
		"format": "json",
	}

	body, err := c.doRequest(ctx, http.MethodPut, "/admin/bucket", query, nil)
	if err != nil {
		return nil, err
	}

	var info BucketInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("unmarshal bucket: %w", err)
	}

	return &info, nil
}

func (c *Client) ReadBucket(ctx context.Context, bucket string) (*BucketInfo, error) {
	query := map[string]string{
		"bucket": bucket,
		"format": "json",
	}

	body, err := c.doRequest(ctx, http.MethodGet, "/admin/bucket", query, nil)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchBucket") {
			return nil, fmt.Errorf("not found")
		}
		return nil, err
	}

	if !json.Valid(body) {
		return nil, fmt.Errorf("RGW returned non-JSON for bucket %q: %s", bucket, string(body))
	}

	var info BucketInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("unmarshal bucket: %w", err)
	}

	return &info, nil
}

func (c *Client) DeleteBucket(ctx context.Context, bucket string, purgeObjects bool) error {
	query := map[string]string{
		"bucket": bucket,
		"format": "json",
	}

	if purgeObjects {
		query["purge-objects"] = "true"
	}

	_, err := c.doRequest(ctx, http.MethodDelete, "/admin/bucket", query, nil)
	return err
}

// GetUserQuota fetches user-level quota.
func (c *Client) GetUserQuota(ctx context.Context, uid string) (*QuotaSpec, error) {
	query := map[string]string{
		"uid":        uid,
		"quota-type": string(QuotaTypeUser),
		"quota":      "",
		"format":     "json",
	}

	body, err := c.doRequest(ctx, http.MethodGet, "/admin/user", query, nil)
	if err != nil {
		return nil, err
	}

	var qs QuotaSpec
	if err := json.Unmarshal(body, &qs); err != nil {
		return nil, fmt.Errorf("unmarshal user quota: %w", err)
	}

	return &qs, nil
}

// SetUserQuota sets user-level quota.
func (c *Client) SetUserQuota(ctx context.Context, uid string, quota QuotaSpec) error {
	query := map[string]string{
		"uid":        uid,
		"quota-type": string(QuotaTypeUser),
		"quota":      "",
		"format":     "json",
	}

	return retryConcurrentModification(ctx, func() error {
		_, err := c.doRequest(ctx, http.MethodPut, "/admin/user", query, quota)
		return err
	})

}

// GetBucketQuota fetches bucket-level quota.
func (c *Client) GetBucketQuota(ctx context.Context, uid, bucket string) (*QuotaSpec, error) {
	query := map[string]string{
		"uid":    uid,
		"bucket": bucket,
		"quota":  "",
		"format": "json",
	}

	body, err := c.doRequest(ctx, http.MethodGet, "/admin/bucket", query, nil)
	if err != nil {
		return nil, err
	}

	var qs QuotaSpec
	if err := json.Unmarshal(body, &qs); err != nil {
		return nil, fmt.Errorf("unmarshal bucket quota: %w", err)
	}

	return &qs, nil
}

// SetBucketQuota sets bucket-level quota.
func (c *Client) SetBucketQuota(ctx context.Context, uid, bucket string, quota QuotaSpec) error {
	query := map[string]string{
		"uid":    uid,
		"bucket": bucket,
		"quota":  "",
		"format": "json",
	}

	return retryConcurrentModification(ctx, func() error {
		_, err := c.doRequest(ctx, http.MethodPut, "/admin/bucket", query, quota)
		return err
	})

}
