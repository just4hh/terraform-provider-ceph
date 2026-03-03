package rbd

type Image struct {
	Pool        string
	Name        string
	Size        uint64
	Features    []string
	CreatedAt   string
	ClusterFSID string
}
