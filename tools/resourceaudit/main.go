// Command resourceaudit validates and summarizes native Linux NATS resource samples.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

const summarySchema = "rabbit-jetstream.io/resource-audit/v1alpha1"

type sample struct {
	CapturedAt    time.Time `json:"captured_at"`
	Node          string    `json:"node"`
	Healthy       *bool     `json:"healthy"`
	HostFreeBytes int64     `json:"host_free_bytes"`
	Server        struct {
		CPU            float64 `json:"cpu"`
		Mem            int64   `json:"mem"`
		SlowConsumers  int64   `json:"slow_consumers"`
		StalledClients int64   `json:"stalled_clients"`
		JetStream      struct {
			Meta struct {
				Pending int64 `json:"pending"`
			} `json:"meta"`
			Stats struct {
				Storage int64 `json:"storage"`
			} `json:"stats"`
		} `json:"jetstream"`
	} `json:"server_statistics"`
}

type nodeSummary struct {
	Samples            int     `json:"samples"`
	FirstMemoryBytes   int64   `json:"first_memory_bytes"`
	LastMemoryBytes    int64   `json:"last_memory_bytes"`
	MaxMemoryBytes     int64   `json:"max_memory_bytes"`
	MaxCPUPercent      float64 `json:"max_cpu_percent"`
	MaxStorageBytes    int64   `json:"max_storage_bytes"`
	MaxMetadataPending int64   `json:"max_metadata_pending"`
}

type summary struct {
	Schema           string                 `json:"schema"`
	GeneratedAt      time.Time              `json:"generated_at"`
	FirstCapturedAt  time.Time              `json:"first_captured_at"`
	LastCapturedAt   time.Time              `json:"last_captured_at"`
	Samples          int                    `json:"samples"`
	MinHostFreeBytes int64                  `json:"min_host_free_bytes"`
	Nodes            map[string]nodeSummary `json:"nodes"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("resourceaudit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("input", "", "resource sampler NDJSON")
	output := flags.String("output", "", "new summary JSON; stdout when empty")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *input == "" || flags.NArg() != 0 {
		return errors.New("-input is required")
	}
	result, err := audit(*input)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if *output == "" {
		_, err = stdout.Write(encoded)
		return err
	}
	file, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(encoded); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func audit(path string) (summary, error) {
	file, err := os.Open(path)
	if err != nil {
		return summary{}, err
	}
	defer file.Close()
	result := summary{Schema: summarySchema, GeneratedAt: time.Now().UTC(), Nodes: map[string]nodeSummary{}}
	lastByNode := map[string]time.Time{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var current sample
		if err := json.Unmarshal(scanner.Bytes(), &current); err != nil {
			return summary{}, fmt.Errorf("decode sample %d: %w", result.Samples+1, err)
		}
		if current.CapturedAt.IsZero() || current.Node == "" || current.Healthy == nil || current.HostFreeBytes <= 0 || current.Server.Mem <= 0 {
			return summary{}, fmt.Errorf("sample %d has incomplete provenance or measurements", result.Samples+1)
		}
		if !*current.Healthy || current.Server.SlowConsumers != 0 || current.Server.StalledClients != 0 {
			return summary{}, fmt.Errorf("sample %d node %s failed health/consumer gate", result.Samples+1, current.Node)
		}
		if previous := lastByNode[current.Node]; !previous.IsZero() && !current.CapturedAt.After(previous) {
			return summary{}, fmt.Errorf("node %s sample timestamps are not increasing", current.Node)
		}
		lastByNode[current.Node] = current.CapturedAt
		node := result.Nodes[current.Node]
		if node.Samples == 0 {
			node.FirstMemoryBytes = current.Server.Mem
		}
		node.Samples++
		node.LastMemoryBytes = current.Server.Mem
		node.MaxMemoryBytes = max(node.MaxMemoryBytes, current.Server.Mem)
		node.MaxCPUPercent = max(node.MaxCPUPercent, current.Server.CPU)
		node.MaxStorageBytes = max(node.MaxStorageBytes, current.Server.JetStream.Stats.Storage)
		node.MaxMetadataPending = max(node.MaxMetadataPending, current.Server.JetStream.Meta.Pending)
		result.Nodes[current.Node] = node
		result.Samples++
		if result.FirstCapturedAt.IsZero() || current.CapturedAt.Before(result.FirstCapturedAt) {
			result.FirstCapturedAt = current.CapturedAt
		}
		if result.LastCapturedAt.IsZero() || current.CapturedAt.After(result.LastCapturedAt) {
			result.LastCapturedAt = current.CapturedAt
		}
		if result.MinHostFreeBytes == 0 || current.HostFreeBytes < result.MinHostFreeBytes {
			result.MinHostFreeBytes = current.HostFreeBytes
		}
	}
	if err := scanner.Err(); err != nil {
		return summary{}, err
	}
	if len(result.Nodes) != 3 {
		keys := make([]string, 0, len(result.Nodes))
		for node := range result.Nodes {
			keys = append(keys, node)
		}
		sort.Strings(keys)
		return summary{}, fmt.Errorf("expected exactly three nodes, got %v", keys)
	}
	counts := -1
	for _, node := range result.Nodes {
		if counts == -1 {
			counts = node.Samples
		} else if node.Samples != counts {
			return summary{}, errors.New("node sample counts differ")
		}
	}
	if counts < 2 {
		return summary{}, errors.New("at least two samples per node are required")
	}
	return result, nil
}

func max[T int64 | float64](left, right T) T {
	if right > left {
		return right
	}
	return left
}
