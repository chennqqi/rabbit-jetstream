package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type sample struct {
	CapturedAt       time.Time      `json:"captured_at"`
	Node             string         `json:"node"`
	Healthy          bool           `json:"healthy"`
	HostFreeBytes    uint64         `json:"host_free_bytes"`
	ServerStatistics map[string]any `json:"server_statistics,omitempty"`
	Error            string         `json:"error,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	output := flag.String("output", "", "new NDJSON output")
	nodes := flag.String("nodes", "", "comma-separated name=monitor-URL entries")
	duration := flag.Duration("duration", 24*time.Hour, "sampling duration")
	interval := flag.Duration("interval", time.Minute, "sampling interval")
	diskPath := flag.String("disk-path", "/", "host filesystem path to sample")
	flag.Parse()
	if *output == "" || *duration <= 0 || *interval <= 0 || *interval > *duration {
		return errors.New("invalid sampler arguments")
	}
	endpoints, err := parseNodes(*nodes)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(file)
	defer file.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	started := time.Now()
	for {
		capturedAt := time.Now().UTC()
		free, statErr := freeBytes(*diskPath)
		for name, endpoint := range endpoints {
			value := collect(client, name, endpoint, capturedAt, free, statErr)
			data, marshalErr := json.Marshal(value)
			if marshalErr != nil {
				return marshalErr
			}
			if _, err := writer.Write(append(data, '\n')); err != nil {
				return err
			}
		}
		if err := writer.Flush(); err != nil {
			return err
		}
		if time.Since(started) >= *duration {
			return file.Sync()
		}
		time.Sleep(*interval)
	}
}

func parseNodes(value string) (map[string]string, error) {
	result := map[string]string{}
	for _, entry := range strings.Split(value, ",") {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || result[parts[0]] != "" {
			return nil, errors.New("invalid node endpoints")
		}
		result[parts[0]] = strings.TrimRight(parts[1], "/")
	}
	if len(result) != 3 {
		return nil, errors.New("exactly three nodes are required")
	}
	return result, nil
}

func collect(client *http.Client, name, endpoint string, capturedAt time.Time, free uint64, statErr error) sample {
	result := sample{CapturedAt: capturedAt, Node: name, HostFreeBytes: free}
	if statErr != nil {
		result.Error = statErr.Error()
		return result
	}
	response, err := client.Get(endpoint + "/varz")
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		result.Error = response.Status
		return result
	}
	if err := json.NewDecoder(response.Body).Decode(&result.ServerStatistics); err != nil {
		result.Error = err.Error()
		return result
	}
	result.Healthy = true
	return result
}
