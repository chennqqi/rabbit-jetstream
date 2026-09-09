package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("perfevidence", flag.ContinueOnError)
	flags.SetOutput(stderr)
	evidencePath := flags.String("evidence", "", "performance evidence manifest")
	createInaugural := flags.Bool("create-inaugural", false, "create and verify first-release native Linux soak evidence")
	reportPath := flags.String("report", "", "candidate performance report")
	resourcesPath := flags.String("resources", "", "resource sampler NDJSON")
	preflightPath := flags.String("native-preflight", "", "native Linux preflight JSON")
	natsImageID := flags.String("nats-image-id", "", "immutable NATS image ID")
	natsBinary := flags.String("nats-binary-sha256", "", "bare-metal NATS binary SHA-256; excludes image ID")
	commandJSON := flags.String("command-json", "", "exact workload command as a JSON string array")
	commandFile := flags.String("command-file", "", "file containing exact workload command as a JSON string array")
	sampleInterval := flags.Int("sample-interval", 60, "resource sample interval in seconds")
	minPublish := flags.Float64("min-publish", 0, "inaugural minimum publish messages/second")
	minConsume := flags.Float64("min-consume", 0, "inaugural minimum consume messages/second")
	maxP99 := flags.Float64("max-p99", 0, "inaugural maximum publish P99 milliseconds")
	requireSoak := flags.Bool("require-soak", false, "require release-grade 24-hour soak evidence")
	sourceRevision := flags.String("source-revision", "", "expected 40-character source revision")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *evidencePath == "" {
		fmt.Fprintln(stderr, "-evidence is required")
		return 2
	}
	if *createInaugural {
		revision, err := createInauguralEvidence(generateOptions{
			EvidencePath: *evidencePath, ReportPath: *reportPath, ResourcesPath: *resourcesPath,
			PreflightPath: *preflightPath, NATSImageID: *natsImageID, CommandJSON: *commandJSON, CommandFile: *commandFile,
			NATSBinarySHA256: *natsBinary,
			SampleInterval:   *sampleInterval, MinPublish: *minPublish, MinConsume: *minConsume, MaxP99: *maxP99,
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := verifyEvidence(*evidencePath, true, revision); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "inaugural performance evidence created and verified")
		return 0
	}
	if err := verifyEvidence(*evidencePath, *requireSoak, *sourceRevision); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "performance evidence verified")
	return 0
}
