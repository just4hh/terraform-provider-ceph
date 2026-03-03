package rgwadmin

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// signV2 signs the request using AWS S3 Signature Version 2.
func (c *Client) signV2(req *http.Request) {
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)

	contentMD5 := req.Header.Get("Content-MD5")

	// For SigV2, Content-Type participates in the string-to-sign when there is a real body.
	// Do not rely on req.GetBody being non-nil; include Content-Type when a body exists and header is set.
	// contentType := ""
	// if req.Body != nil {
	// 	contentType = req.Header.Get("Content-Type")
	// }
	contentType := req.Header.Get("Content-Type")

	canonicalResource := canonicalAdminResource(req.URL)

	stringToSign := strings.Join([]string{
		req.Method,
		contentMD5,
		contentType,
		date,
		canonicalResource,
	}, "\n")

	// log.Printf("SigV2 stringToSign:\n%s", stringToSign)
	// fmt.Printf("AccessKey=%q SecretKeyLen=%d\n", c.AccessKey, len(c.SecretKey))

	mac := hmac.New(sha1.New, []byte(c.SecretKey))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	// log.Printf("Authorization header: %q", req.Header.Get("Authorization"))
	// fmt.Printf("secret bytes: %x\n", []byte(c.SecretKey))
	// fmt.Printf("stringToSign bytes: %x\n", []byte(stringToSign))

	// fmt.Println(signature) // should equal the part after "AWS ACCESSKEY:" in Authorization

	req.Header.Set("Authorization", "AWS "+c.AccessKey+":"+signature)
}

// canonicalAdminResource builds the canonical resource for RGW admin API.
// RGW Admin API does NOT use S3 subresource canonicalization.
// Query parameters like uid, format, bucket, etc. MUST NOT appear.
func canonicalAdminResource(u *url.URL) string {
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	return path
}
