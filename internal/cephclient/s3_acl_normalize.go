package cephclient

import (
	"sort"
	"strings"
)

// normalizeS3ACL sorts and normalizes ACL grants to avoid noisy diffs.
func normalizeS3ACL(acl *S3ACL) {
	if acl == nil {
		return
	}

	// Normalize fields before sorting.
	for i := range acl.Grants {
		g := &acl.Grants[i]
		g.GranteeType = strings.TrimSpace(g.GranteeType)
		g.ID = strings.TrimSpace(g.ID)
		g.URI = strings.TrimSpace(strings.ToLower(g.URI))
		g.Permission = strings.TrimSpace(g.Permission)
	}

	sort.Slice(acl.Grants, func(i, j int) bool {
		a, b := acl.Grants[i], acl.Grants[j]
		if a.GranteeType != b.GranteeType {
			return a.GranteeType < b.GranteeType
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.URI != b.URI {
			return a.URI < b.URI
		}
		return a.Permission < b.Permission
	})
}
