package main

import (
	"bytes"
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
		fmt.Fprintln(stdout, "Usage: rjsctl status [--url URL] | queue list [--url URL] | queue validate FILE | queue plan FILE | queue diff CURRENT DESIRED | queue reconcile [--url URL] FILE | queue apply [--url URL] [--token TOKEN] FILE | queue delete [--url URL] [--token TOKEN] --confirm NAME [--force] NAME | version")
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
	if len(args) < 1 {
		return errors.New("usage: rjsctl queue list [--url URL] | queue validate FILE | queue plan FILE | queue diff CURRENT DESIRED | queue reconcile [--url URL] FILE | queue apply [--url URL] [--token TOKEN] FILE")
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("queue list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		baseURL := fs.String("url", "http://127.0.0.1:8223", "management API base URL")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("usage: rjsctl queue list [--url URL]")
		}
		client := &http.Client{Timeout: 5 * time.Second}
		var declarations any
		status, err := getJSON(client, strings.TrimRight(*baseURL, "/")+"/api/v1/queues?limit=200", &declarations)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("management API returned %d", status)
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(declarations)
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
	case "apply":
		fs := flag.NewFlagSet("queue apply", flag.ContinueOnError)
		fs.SetOutput(stderr)
		baseURL := fs.String("url", "http://127.0.0.1:8223", "management API base URL")
		token := fs.String("token", os.Getenv("RJS_ADMIN_TOKEN"), "management API bearer token")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return errors.New("usage: rjsctl queue apply [--url URL] [--token TOKEN] FILE")
		}
		if *token == "" {
			return errors.New("queue apply requires --token or RJS_ADMIN_TOKEN")
		}
		queue, err := readQueue(fs.Arg(0))
		if err != nil {
			return err
		}
		body, err := json.Marshal(queue)
		if err != nil {
			return err
		}
		endpoint := strings.TrimRight(*baseURL, "/") + "/api/v1/queues/" + url.PathEscape(queue.Metadata.Name)
		client := &http.Client{Timeout: 15 * time.Second}
		ifMatch, ifNoneMatch, err := queueRevisionHeaders(client, endpoint)
		if err != nil {
			return err
		}
		request, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+*token)
		request.Header.Set("Content-Type", "application/json")
		if ifMatch != "" {
			request.Header.Set("If-Match", ifMatch)
		} else {
			request.Header.Set("If-None-Match", ifNoneMatch)
		}
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("management API returned %s: %s", response.Status, readErrorBody(response.Body))
		}
		var result topology.ReconcileResult
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	case "delete":
		fs := flag.NewFlagSet("queue delete", flag.ContinueOnError)
		fs.SetOutput(stderr)
		baseURL := fs.String("url", "http://127.0.0.1:8223", "management API base URL")
		token := fs.String("token", os.Getenv("RJS_ADMIN_TOKEN"), "management API bearer token")
		confirm := fs.String("confirm", "", "Queue name confirmation")
		force := fs.Bool("force", false, "delete a Stream containing messages")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return errors.New("usage: rjsctl queue delete [--url URL] [--token TOKEN] --confirm NAME [--force] NAME")
		}
		name := fs.Arg(0)
		if !topology.ValidQueueName(name) {
			return errors.New("invalid Queue name")
		}
		if *token == "" {
			return errors.New("queue delete requires --token or RJS_ADMIN_TOKEN")
		}
		if *confirm != name {
			return errors.New("--confirm must exactly match the Queue name")
		}
		endpoint := strings.TrimRight(*baseURL, "/") + "/api/v1/queues/" + url.PathEscape(name)
		client := &http.Client{Timeout: 15 * time.Second}
		ifMatch, ifNoneMatch, err := queueRevisionHeaders(client, endpoint)
		if err != nil {
			return err
		}
		if *force {
			endpoint += "?force=true"
		}
		request, err := http.NewRequest(http.MethodDelete, endpoint, nil)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+*token)
		request.Header.Set("X-RJS-Confirm-Queue", name)
		if ifMatch != "" {
			request.Header.Set("If-Match", ifMatch)
		} else {
			request.Header.Set("If-None-Match", ifNoneMatch)
		}
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("management API returned %s: %s", response.Status, readErrorBody(response.Body))
		}
		var result topology.DeleteResult
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	default:
		return fmt.Errorf("unknown queue command %q", args[0])
	}
}

func queueRevisionHeaders(client *http.Client, endpoint string) (string, string, error) {
	lookup, err := client.Get(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("read Queue revision: %w", err)
	}
	defer lookup.Body.Close()
	switch lookup.StatusCode {
	case http.StatusOK:
		etag := lookup.Header.Get("ETag")
		if etag == "" {
			return "", "", errors.New("management API did not return Queue ETag")
		}
		return etag, "", nil
	case http.StatusNotFound:
		return "", "*", nil
	default:
		return "", "", fmt.Errorf("read Queue revision: management API returned %s", lookup.Status)
	}
}

func readErrorBody(reader io.Reader) string {
	value, err := io.ReadAll(io.LimitReader(reader, 4096))
	if err != nil {
		return "unreadable response"
	}
	return strings.TrimSpace(string(value))
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
