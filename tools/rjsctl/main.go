package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(stdout, "Usage: rjsctl status [--url URL] | queue validate FILE | queue plan FILE | queue diff CURRENT DESIRED | queue reconcile [--url URL] FILE | version")
		return nil
	}
	if args[0] == "version" {
		fmt.Fprintln(stdout, version)
		return nil
	}
	if args[0] == "queue" {
		return runQueue(args[1:], stdout, stderr)
	}
	if args[0] != "status" {
		return fmt.Errorf("unknown command %q", args[0])
	}
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baseURL := fs.String("url", "http://127.0.0.1:8223", "management API base URL")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get(*baseURL + "/api/v1/info")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("management API returned %s", response.Status)
	}
	var value any
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		return err
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func runQueue(args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: rjsctl queue validate FILE | queue plan FILE | queue diff CURRENT DESIRED | queue reconcile [--url URL] FILE")
	}
	switch args[0] {
	case "validate":
		if len(args) != 2 {
			return errors.New("usage: rjsctl queue validate FILE")
		}
		queue, err := readQueue(args[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "valid Queue %s (%s)\n", queue.Metadata.Name, queue.APIVersion)
		return nil
	case "diff":
		if len(args) != 3 {
			return errors.New("usage: rjsctl queue diff CURRENT DESIRED")
		}
		current, err := readQueue(args[1])
		if err != nil {
			return fmt.Errorf("current queue: %w", err)
		}
		desired, err := readQueue(args[2])
		if err != nil {
			return fmt.Errorf("desired queue: %w", err)
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(topology.Compare(*current, *desired))
	case "plan":
		if len(args) != 2 {
			return errors.New("usage: rjsctl queue plan FILE")
		}
		queue, err := readQueue(args[1])
		if err != nil {
			return err
		}
		plan, err := topology.BuildPlan(*queue)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	case "reconcile":
		fs := flag.NewFlagSet("queue reconcile", flag.ContinueOnError)
		fs.SetOutput(stderr)
		baseURL := fs.String("url", "http://127.0.0.1:8223", "management API base URL")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return errors.New("usage: rjsctl queue reconcile [--url URL] FILE")
		}
		queue, err := readQueue(fs.Arg(0))
		if err != nil {
			return err
		}
		plan, err := topology.BuildPlan(*queue)
		if err != nil {
			return err
		}
		observed, err := readObservedTopology(*baseURL, plan)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(topology.Reconcile(plan, observed))
	default:
		return fmt.Errorf("unknown queue command %q", args[0])
	}
}

func readObservedTopology(baseURL string, plan topology.Plan) (topology.ObservedTopology, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	baseURL = strings.TrimRight(baseURL, "/")
	streamPath := "/api/v1/streams/" + url.PathEscape(plan.Stream.Name)
	var observed topology.ObservedTopology
	status, err := getJSON(client, baseURL+streamPath, &observed.Stream)
	if err != nil {
		return observed, fmt.Errorf("read Stream state: %w", err)
	}
	if status == http.StatusNotFound {
		observed.Stream = nil
		return observed, nil
	}
	if status != http.StatusOK {
		return observed, fmt.Errorf("read Stream state: management API returned %d", status)
	}
	var consumers struct {
		Items []topology.ObservedConsumer `json:"items"`
	}
	status, err = getJSON(client, baseURL+streamPath+"/consumers?limit=200", &consumers)
	if err != nil {
		return observed, fmt.Errorf("read Consumer state: %w", err)
	}
	if status != http.StatusOK {
		return observed, fmt.Errorf("read Consumer state: management API returned %d", status)
	}
	for index := range consumers.Items {
		if consumers.Items[index].Name == plan.Consumer.Name {
			observed.Consumer = &consumers.Items[index]
			break
		}
	}
	return observed, nil
}

func getJSON(client *http.Client, endpoint string, target any) (int, error) {
	response, err := client.Get(endpoint)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		if err := json.NewDecoder(response.Body).Decode(target); err != nil {
			return response.StatusCode, err
		}
	}
	return response.StatusCode, nil
}

func readQueue(path string) (*topology.Queue, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	queue, err := topology.ParseQueue(file)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return queue, nil
}
