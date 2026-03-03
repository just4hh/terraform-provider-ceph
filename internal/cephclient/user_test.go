package cephclient

import "testing"

func TestParseCapsVariants(t *testing.T) {
	cases := []struct {
		name string
		raw  any
		want map[string]string
	}{
		{
			"map",
			map[string]any{"mon": "allow r", "osd": "allow rwx pool=rbd"},
			map[string]string{"mon": "allow r", "osd": "allow rwx pool=rbd"},
		},
		{
			"pairs",
			[]any{[]any{"mon", "allow r"}, []any{"osd", "allow rwx pool=rbd"}},
			map[string]string{"mon": "allow r", "osd": "allow rwx pool=rbd"},
		},
		{
			"maps",
			[]any{map[string]any{"mon": "allow r"}, map[string]any{"osd": "allow rwx pool=rbd"}},
			map[string]string{"mon": "allow r", "osd": "allow rwx pool=rbd"},
		},
		{
			"flat",
			[]any{"mon", "allow r", "osd", "allow rwx pool=rbd"},
			map[string]string{"mon": "allow r", "osd": "allow rwx pool=rbd"},
		},
	}

	for _, tc := range cases {
		got := parseCaps(tc.raw)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: length mismatch got=%v want=%v", tc.name, got, tc.want)
		}
		for k, v := range tc.want {
			if got[k] != v {
				t.Fatalf("%s: key %s mismatch got=%q want=%q", tc.name, k, got[k], v)
			}
		}
	}
}
