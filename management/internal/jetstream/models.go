package jetstream

import "time"

type Account struct {
	Domain         string `json:"domain,omitempty"`
	MemoryUsed     uint64 `json:"memory_used"`
	StorageUsed    uint64 `json:"storage_used"`
	ReservedMemory uint64 `json:"reserved_memory"`
	ReservedStore  uint64 `json:"reserved_storage"`
	Streams        int    `json:"streams"`
	Consumers      int    `json:"consumers"`
	APITotal       uint64 `json:"api_total"`
	APIErrors      uint64 `json:"api_errors"`
	APIInflight    uint64 `json:"api_inflight"`
	APILevel       int    `json:"api_level"`
}

type Stream struct {
	Name      string       `json:"name"`
	Subjects  []string     `json:"subjects"`
	Storage   string       `json:"storage"`
	Replicas  int          `json:"replicas"`
	Created   time.Time    `json:"created"`
	Messages  uint64       `json:"messages"`
	Bytes     uint64       `json:"bytes"`
	Consumers int          `json:"consumers"`
	FirstSeq  uint64       `json:"first_sequence"`
	LastSeq   uint64       `json:"last_sequence"`
	Cluster   *ClusterInfo `json:"cluster,omitempty"`
}

type ClusterInfo struct {
	Name     string    `json:"name,omitempty"`
	Leader   string    `json:"leader,omitempty"`
	Replicas []Replica `json:"replicas"`
}

type Replica struct {
	Name        string `json:"name"`
	Current     bool   `json:"current"`
	Offline     bool   `json:"offline"`
	Lag         uint64 `json:"lag"`
	ActiveNanos int64  `json:"active_nanos"`
}

type Consumer struct {
	Stream        string       `json:"stream"`
	Name          string       `json:"name"`
	Created       time.Time    `json:"created"`
	Durable       string       `json:"durable,omitempty"`
	FilterSubject string       `json:"filter_subject,omitempty"`
	AckPolicy     string       `json:"ack_policy"`
	Pending       uint64       `json:"pending"`
	AckPending    int          `json:"ack_pending"`
	Redelivered   int          `json:"redelivered"`
	Waiting       int          `json:"waiting"`
	Delivered     uint64       `json:"delivered"`
	Cluster       *ClusterInfo `json:"cluster,omitempty"`
}
