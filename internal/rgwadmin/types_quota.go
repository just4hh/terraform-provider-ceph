package rgwadmin

type QuotaType string

const (
	QuotaTypeUser   QuotaType = "user"
	QuotaTypeBucket QuotaType = "bucket"
)

type QuotaSpec struct {
	Enabled    bool  `json:"enabled"`
	MaxSizeKb  int64 `json:"max_size_kb"`
	MaxObjects int64 `json:"max_objects"`
}
