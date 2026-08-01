package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type Client struct {
	endpoints []string
	http      *http.Client
}

type Snapshot struct {
	Status      string `json:"status"`
	Total       int    `json:"total"`
	Available   int    `json:"available"`
	Degraded    int    `json:"degraded"`
	Unavailable int    `json:"unavailable"`
	Nodes       []Node `json:"nodes"`
}

type Node struct {
	Endpoint       string    `json:"endpoint"`
	Status         string    `json:"status"`
	Errors         []string  `json:"errors,omitempty"`
	ObservedAt     time.Time `json:"observed_at,omitempty"`
	ID             string    `json:"id,omitempty"`
	Name           string    `json:"name,omitempty"`
	Version        string    `json:"version,omitempty"`
	GoVersion      string    `json:"go_version,omitempty"`
	Uptime         string    `json:"uptime,omitempty"`
	MemoryBytes    int64     `json:"memory_bytes,omitempty"`
	CPUPercent     float64   `json:"cpu_percent,omitempty"`
	Cores          int       `json:"cores,omitempty"`
	Connections    int       `json:"connections,omitempty"`
	Subscriptions  uint32    `json:"subscriptions,omitempty"`
	SlowConsumers  int64     `json:"slow_consumers,omitempty"`
	InMessages     int64     `json:"in_messages,omitempty"`
	OutMessages    int64     `json:"out_messages,omitempty"`
	ClusterName    string    `json:"cluster_name,omitempty"`
	ConnectedPeers []string  `json:"connected_peers"`
	JetStream      JSState   `json:"jetstream"`
}

type JSState struct {
	Enabled         bool   `json:"enabled"`
	MemoryBytes     uint64 `json:"memory_bytes"`
	StorageBytes    uint64 `json:"storage_bytes"`
	Streams         int    `json:"streams"`
	Consumers       int    `json:"consumers"`
	Messages        uint64 `json:"messages"`
	MetaLeader      string `json:"meta_leader,omitempty"`
	MetaClusterSize int    `json:"meta_cluster_size,omitempty"`
	MetaPending     int    `json:"meta_pending,omitempty"`
}

type varz struct {
	ID            string    `json:"server_id"`
	Name          string    `json:"server_name"`
	Version       string    `json:"version"`
	GoVersion     string    `json:"go"`
	Now           time.Time `json:"now"`
	Uptime        string    `json:"uptime"`
	Mem           int64     `json:"mem"`
	CPU           float64   `json:"cpu"`
	Cores         int       `json:"cores"`
	Connections   int       `json:"connections"`
	Subscriptions uint32    `json:"subscriptions"`
	SlowConsumers int64     `json:"slow_consumers"`
	InMsgs        int64     `json:"in_msgs"`
	OutMsgs       int64     `json:"out_msgs"`
	Cluster       struct {
		Name string `json:"name"`
	} `json:"cluster"`
	JetStream struct {
		Config *json.RawMessage `json:"config"`
	} `json:"jetstream"`
}

type routez struct {
	Routes []struct {
		RemoteName string `json:"remote_name"`
	} `json:"routes"`
}

type jsz struct {
	Memory    uint64 `json:"memory"`
	Storage   uint64 `json:"storage"`
	Streams   int    `json:"streams"`
	Consumers int    `json:"consumers"`
	Messages  uint64 `json:"messages"`
	Meta      struct {
		Leader      string `json:"leader"`
		ClusterSize int    `json:"cluster_size"`
		Pending     int    `json:"pending"`
	} `json:"meta_cluster"`
}

func New(endpoints string, timeout time.Duration) *Client {
	values := make([]string, 0)
	for _, endpoint := range strings.Split(endpoints, ",") {
		if endpoint = strings.TrimSpace(endpoint); endpoint != "" {
			values = append(values, strings.TrimRight(endpoint, "/"))
		}
	}
	return &Client{endpoints: values, http: &http.Client{Timeout: timeout}}
}

func (c *Client) Nodes(ctx context.Context) Snapshot {
	nodes := make([]Node, len(c.endpoints))
	var wg sync.WaitGroup
	for index, endpoint := range c.endpoints {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nodes[index] = c.node(ctx, endpoint)
		}()
	}
	wg.Wait()
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Name == nodes[j].Name {
			return nodes[i].Endpoint < nodes[j].Endpoint
		}
		return nodes[i].Name < nodes[j].Name
	})
	snapshot := Snapshot{Total: len(nodes), Nodes: nodes}
	for _, node := range nodes {
		switch node.Status {
		case "available":
			snapshot.Available++
		case "degraded":
			snapshot.Degraded++
		default:
			snapshot.Unavailable++
		}
	}
	switch {
	case snapshot.Unavailable == snapshot.Total && snapshot.Total > 0:
		snapshot.Status = "unavailable"
	case snapshot.Unavailable > 0 || snapshot.Degraded > 0:
		snapshot.Status = "degraded"
	default:
		snapshot.Status = "available"
	}
	return snapshot
}

func (c *Client) node(ctx context.Context, endpoint string) Node {
	node := Node{Endpoint: publicEndpoint(endpoint), Status: "available", ConnectedPeers: []string{}}
	var server varz
	if err := c.get(ctx, endpoint+"/varz", &server); err != nil {
		node.Status = "unavailable"
		node.Errors = append(node.Errors, "varz: "+safeError(err, endpoint))
		return node
	}
	node.ID, node.Name, node.Version, node.GoVersion = server.ID, server.Name, server.Version, server.GoVersion
	node.ObservedAt, node.Uptime, node.MemoryBytes, node.CPUPercent = server.Now, server.Uptime, server.Mem, server.CPU
	node.Cores, node.Connections, node.Subscriptions = server.Cores, server.Connections, server.Subscriptions
	node.SlowConsumers, node.InMessages, node.OutMessages, node.ClusterName = server.SlowConsumers, server.InMsgs, server.OutMsgs, server.Cluster.Name
	node.JetStream.Enabled = server.JetStream.Config != nil

	var routes routez
	if err := c.get(ctx, endpoint+"/routez", &routes); err != nil {
		node.Status = "degraded"
		node.Errors = append(node.Errors, "routez: "+safeError(err, endpoint))
	} else {
		peers := make(map[string]struct{})
		for _, route := range routes.Routes {
			if route.RemoteName != "" && route.RemoteName != node.Name {
				peers[route.RemoteName] = struct{}{}
			}
		}
		for peer := range peers {
			node.ConnectedPeers = append(node.ConnectedPeers, peer)
		}
		sort.Strings(node.ConnectedPeers)
	}

	var state jsz
	if err := c.get(ctx, endpoint+"/jsz", &state); err != nil {
		node.Status = "degraded"
		node.Errors = append(node.Errors, "jsz: "+safeError(err, endpoint))
	} else {
		node.JetStream.MemoryBytes, node.JetStream.StorageBytes = state.Memory, state.Storage
		node.JetStream.Streams, node.JetStream.Consumers, node.JetStream.Messages = state.Streams, state.Consumers, state.Messages
		node.JetStream.MetaLeader, node.JetStream.MetaClusterSize, node.JetStream.MetaPending = state.Meta.Leader, state.Meta.ClusterSize, state.Meta.Pending
	}
	return node
}

func (c *Client) get(ctx context.Context, endpoint string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("returned %s", response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func publicEndpoint(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	parsed.User = nil
	return parsed.String()
}

func safeError(err error, endpoint string) string {
	return strings.ReplaceAll(err.Error(), endpoint, publicEndpoint(endpoint))
}
