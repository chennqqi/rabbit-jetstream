package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	Sources        map[string]SourceObservation `json:"sources"`
	Endpoint       string                       `json:"endpoint"`
	Status         string                       `json:"status"`
	Errors         []string                     `json:"errors,omitempty"`
	ObservedAt     time.Time                    `json:"observed_at,omitempty"`
	ID             string                       `json:"id,omitempty"`
	Name           string                       `json:"name,omitempty"`
	Version        string                       `json:"version,omitempty"`
	GoVersion      string                       `json:"go_version,omitempty"`
	Uptime         string                       `json:"uptime,omitempty"`
	MemoryBytes    *int64                       `json:"memory_bytes,omitempty"`
	CPUPercent     *float64                     `json:"cpu_percent,omitempty"`
	Cores          *int                         `json:"cores,omitempty"`
	Connections    *int                         `json:"connections,omitempty"`
	Subscriptions  *uint32                      `json:"subscriptions,omitempty"`
	SlowConsumers  *int64                       `json:"slow_consumers,omitempty"`
	InMessages     *int64                       `json:"in_messages,omitempty"`
	OutMessages    *int64                       `json:"out_messages,omitempty"`
	ClusterName    string                       `json:"cluster_name,omitempty"`
	ConnectedPeers []string                     `json:"connected_peers"`
	JetStream      JSState                      `json:"jetstream"`
}

// Source availability is distinct from server health and zero-valued metrics.
type SourceObservation struct {
	Available bool      `json:"available"`
	ReadAt    time.Time `json:"read_at"`
}

type JSState struct {
	Enabled         *bool   `json:"enabled,omitempty"`
	MemoryBytes     *uint64 `json:"memory_bytes,omitempty"`
	StorageBytes    *uint64 `json:"storage_bytes,omitempty"`
	Streams         *int    `json:"streams,omitempty"`
	Consumers       *int    `json:"consumers,omitempty"`
	Messages        *uint64 `json:"messages,omitempty"`
	MetaLeader      string  `json:"meta_leader,omitempty"`
	MetaClusterSize *int    `json:"meta_cluster_size,omitempty"`
	MetaPending     *int    `json:"meta_pending,omitempty"`
}

type varz struct {
	ID            string    `json:"server_id"`
	Name          string    `json:"server_name"`
	Version       string    `json:"version"`
	GoVersion     string    `json:"go"`
	Now           time.Time `json:"now"`
	Uptime        string    `json:"uptime"`
	Mem           *int64    `json:"mem"`
	CPU           *float64  `json:"cpu"`
	Cores         *int      `json:"cores"`
	Connections   *int      `json:"connections"`
	Subscriptions *uint32   `json:"subscriptions"`
	SlowConsumers *int64    `json:"slow_consumers"`
	InMsgs        *int64    `json:"in_msgs"`
	OutMsgs       *int64    `json:"out_msgs"`
	Cluster       struct {
		Name string `json:"name"`
	} `json:"cluster"`
	JetStream struct {
		Config map[string]json.RawMessage `json:"config"`
	} `json:"jetstream"`
}

type routez struct {
	ID        string `json:"server_id"`
	NumRoutes *int   `json:"num_routes"`
	Routes    []struct {
		RemoteName string `json:"remote_name"`
	} `json:"routes"`
}

type jsz struct {
	Disabled  *bool   `json:"disabled"`
	ID        string  `json:"server_id"`
	Memory    *uint64 `json:"memory"`
	Storage   *uint64 `json:"storage"`
	Streams   *int    `json:"streams"`
	Consumers *int    `json:"consumers"`
	Messages  *uint64 `json:"messages"`
	Meta      struct {
		Leader      string `json:"leader"`
		ClusterSize *int   `json:"cluster_size"`
		Pending     *int   `json:"pending"`
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
	node := Node{Endpoint: publicEndpoint(endpoint), Status: "available", ConnectedPeers: []string{}, Sources: make(map[string]SourceObservation)}
	var server varz
	err := c.get(ctx, endpoint+"/varz", &server)
	if err == nil && strings.TrimSpace(server.ID) == "" {
		err = fmt.Errorf("missing server identity")
	}
	if err != nil {
		node.Sources["varz"] = SourceObservation{ReadAt: time.Now().UTC()}
		node.Status = "unavailable"
		node.Errors = append(node.Errors, "varz: "+safeError(err, endpoint))
		return node
	}
	node.Sources["varz"] = SourceObservation{Available: true, ReadAt: time.Now().UTC()}
	node.ID, node.Name, node.Version, node.GoVersion = server.ID, server.Name, server.Version, server.GoVersion
	node.ObservedAt, node.Uptime, node.MemoryBytes, node.CPUPercent = server.Now, server.Uptime, server.Mem, server.CPU
	node.Cores, node.Connections, node.Subscriptions = server.Cores, server.Connections, server.Subscriptions
	node.SlowConsumers, node.InMessages, node.OutMessages, node.ClusterName = server.SlowConsumers, server.InMsgs, server.OutMsgs, server.Cluster.Name
	if server.JetStream.Config != nil {
		enabled := true
		node.JetStream.Enabled = &enabled
	}

	var routes routez
	err = c.getForServer(ctx, endpoint+"/routez", &routes, &routes.ID, server.ID)
	if err == nil {
		err = routes.validate()
	}
	if err != nil {
		node.Sources["routez"] = SourceObservation{ReadAt: time.Now().UTC()}
		node.Status = "degraded"
		node.Errors = append(node.Errors, "routez: "+safeError(err, endpoint))
	} else {
		node.Sources["routez"] = SourceObservation{Available: true, ReadAt: time.Now().UTC()}
		peers := make(map[string]struct{})
		for _, route := range routes.Routes {
			if route.RemoteName != "" {
				peers[route.RemoteName] = struct{}{}
			}
		}
		for peer := range peers {
			node.ConnectedPeers = append(node.ConnectedPeers, peer)
		}
		sort.Strings(node.ConnectedPeers)
	}

	var state jsz
	if err := c.getForServer(ctx, endpoint+"/jsz", &state, &state.ID, server.ID); err != nil {
		node.Sources["jsz"] = SourceObservation{ReadAt: time.Now().UTC()}
		node.Status = "degraded"
		node.Errors = append(node.Errors, "jsz: "+safeError(err, endpoint))
	} else {
		node.Sources["jsz"] = SourceObservation{Available: true, ReadAt: time.Now().UTC()}
		if state.Disabled != nil {
			enabled := !*state.Disabled
			if node.JetStream.Enabled != nil && *node.JetStream.Enabled != enabled {
				node.JetStream.Enabled = nil
				node.Status = "degraded"
				node.Errors = append(node.Errors, "jetstream: conflicting enablement observations")
			} else {
				node.JetStream.Enabled = &enabled
			}
		}
		node.JetStream.MemoryBytes, node.JetStream.StorageBytes = state.Memory, state.Storage
		node.JetStream.Streams, node.JetStream.Consumers, node.JetStream.Messages = state.Streams, state.Consumers, state.Messages
		node.JetStream.MetaLeader, node.JetStream.MetaClusterSize, node.JetStream.MetaPending = state.Meta.Leader, state.Meta.ClusterSize, state.Meta.Pending
	}
	return node
}

func (routes routez) validate() error {
	if routes.NumRoutes == nil || *routes.NumRoutes < 0 || routes.Routes == nil || *routes.NumRoutes != len(routes.Routes) {
		return fmt.Errorf("missing or incomplete route observations")
	}
	for _, route := range routes.Routes {
		if strings.TrimSpace(route.RemoteName) == "" {
			return fmt.Errorf("route peer name unavailable")
		}
	}
	return nil
}

// A stable endpoint can still serve different nodes during a restart or behind
// a proxy. Never attach another server's observations to the varz identity.
func (c *Client) getForServer(ctx context.Context, endpoint string, target any, id *string, expected string) error {
	if err := c.get(ctx, endpoint, target); err != nil {
		return err
	}
	if *id != expected {
		return fmt.Errorf("missing or mismatched server identity")
	}
	return nil
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
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	// A valid prefix is not a valid response. Reject trailing values/garbage and
	// body-read errors before the caller publishes any decoded observations.
	if _, err := decoder.Token(); err != io.EOF {
		if err != nil {
			return fmt.Errorf("decode response suffix: %w", err)
		}
		return fmt.Errorf("multiple JSON values in monitoring response")
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
