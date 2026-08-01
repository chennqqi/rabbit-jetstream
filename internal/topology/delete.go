package topology

type DeleteResult struct {
	Queue    string `json:"queue"`
	Stream   string `json:"stream"`
	Status   string `json:"status"`
	Blocked  bool   `json:"blocked"`
	Forced   bool   `json:"forced"`
	Messages uint64 `json:"messages"`
	Reason   string `json:"reason,omitempty"`
}
