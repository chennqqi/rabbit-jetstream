package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type generateOptions struct {
	EvidencePath   string
	ReportPath     string
	ResourcesPath  string
	PreflightPath  string
	NATSImageID    string
	CommandJSON    string
	CommandFile    string
	SampleInterval int
	MinPublish     float64
	MinConsume     float64
	MaxP99         float64
}

type nativePreflight struct {
	Schema         string `json:"schema"`
	SourceRevision string `json:"source_revision"`
	Runtime        string `json:"runtime"`
	Docker         struct {
		OperatingSystem string `json:"OperatingSystem"`
		OSType          string `json:"OSType"`
		Architecture    string `json:"Architecture"`
		KernelVersion   string `json:"KernelVersion"`
		Driver          string `json:"Driver"`
		NCPU            int    `json:"NCPU"`
		MemTotal        int64  `json:"MemTotal"`
	} `json:"docker"`
}

func createInauguralEvidence(options generateOptions) (string, error) {
	if options.EvidencePath == "" || options.ReportPath == "" || options.ResourcesPath == "" || options.PreflightPath == "" || options.SampleInterval < 1 || options.MinPublish <= 0 || options.MinConsume <= 0 || options.MaxP99 <= 0 {
		return "", errors.New("inaugural evidence generation arguments are incomplete")
	}
	if filepath.Dir(options.ReportPath) != filepath.Dir(options.EvidencePath) || filepath.Dir(options.ResourcesPath) != filepath.Dir(options.EvidencePath) {
		return "", errors.New("evidence, report, and resources must be in the same directory")
	}
	imageID := options.NATSImageID
	if len(imageID) == 64 {
		imageID = "sha256:" + imageID
	}
	if len(imageID) != len("sha256:")+64 || !strings.HasPrefix(imageID, "sha256:") || strings.Trim(strings.TrimPrefix(imageID, "sha256:"), "0123456789abcdef") != "" {
		return "", errors.New("NATS image ID must be a lowercase SHA-256 ID")
	}
	if (options.CommandJSON == "") == (options.CommandFile == "") {
		return "", errors.New("exactly one of command-json or command-file is required")
	}
	commandValue := []byte(options.CommandJSON)
	if options.CommandFile != "" {
		value, readErr := os.ReadFile(options.CommandFile)
		if readErr != nil {
			return "", fmt.Errorf("read command file: %w", readErr)
		}
		commandValue = value
	}
	var command []string
	if err := json.Unmarshal(commandValue, &command); err != nil || len(command) == 0 {
		return "", errors.New("workload command must be a non-empty JSON string array")
	}
	var preflight nativePreflight
	if err := decodePreflight(options.PreflightPath, &preflight); err != nil {
		return "", fmt.Errorf("decode native preflight: %w", err)
	}
	if preflight.Schema != "rabbit-jetstream.io/native-linux-preflight/v1alpha1" || preflight.Runtime != "linux/amd64" || len(preflight.SourceRevision) != 40 || preflight.Docker.OSType != "linux" || preflight.Docker.NCPU < 1 || preflight.Docker.MemTotal < 1 {
		return "", errors.New("native preflight does not prove a Linux AMD64 host")
	}
	reportDigest, err := fileDigest(options.ReportPath)
	if err != nil {
		return "", err
	}
	resourcesDigest, err := fileDigest(options.ResourcesPath)
	if err != nil {
		return "", err
	}
	proof := evidence{
		Schema: evidenceSchema, Mode: "soak", GeneratedAt: time.Now().UTC(), SourceRevision: preflight.SourceRevision,
		NATSImageID: imageID, Command: command, SampleIntervalSeconds: options.SampleInterval,
		MaxThroughputRegressionPercent: 20, MaxP99RegressionPercent: 30, BaselineMode: "inaugural",
		MinPublishMessagesPerSecond: options.MinPublish, MinConsumeMessagesPerSecond: options.MinConsume, MaxPublishLatencyP99Millis: options.MaxP99,
		Host: map[string]any{
			"operating_system": preflight.Docker.OperatingSystem, "os_type": preflight.Docker.OSType,
			"architecture": preflight.Docker.Architecture, "kernel_version": preflight.Docker.KernelVersion,
			"cpus": preflight.Docker.NCPU, "memory_bytes": preflight.Docker.MemTotal, "storage_driver": preflight.Docker.Driver,
		},
		Report:    artifact{File: filepath.Base(options.ReportPath), SHA256: reportDigest},
		Resources: artifact{File: filepath.Base(options.ResourcesPath), SHA256: resourcesDigest},
	}
	encoded, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(options.EvidencePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return preflight.SourceRevision, nil
}

func decodePreflight(path string, target *nativePreflight) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON documents are not allowed")
		}
		return fmt.Errorf("trailing JSON: %w", err)
	}
	return nil
}

func fileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
