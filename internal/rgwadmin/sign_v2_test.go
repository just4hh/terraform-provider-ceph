package rgwadmin

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestCanonicalAdminResourceIncludesQueryParams(t *testing.T) {
	u, _ := url.Parse("http://example.local/admin/user?uid=client.backup&format=json")
	got := canonicalAdminResource(u)
	if !strings.Contains(got, "format=json") || !strings.Contains(got, "uid=client.backup") {
		t.Fatalf("unexpected canonical resource: %q", got)
	}
}

func TestStringToSignIncludesContentTypeWhenBodyPresent(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://example.local/admin/user?format=json", bytes.NewReader([]byte(`{"x":"y"}`)))
	req.Header.Set("Content-Type", "application/json")
	// replicate stringToSign builder (or expose helper) and assert it contains "application/json"
	// ...
}
