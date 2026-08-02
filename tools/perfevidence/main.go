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
	requireSoak := flags.Bool("require-soak", false, "require release-grade 24-hour soak evidence")
	sourceRevision := flags.String("source-revision", "", "expected 40-character source revision")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *evidencePath == "" {
		fmt.Fprintln(stderr, "-evidence is required")
		return 2
	}
	if err := verifyEvidence(*evidencePath, *requireSoak, *sourceRevision); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "performance evidence verified")
	return 0
}
