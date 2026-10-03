package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRejectsArguments(t *testing.T) {
	if out, err := exec.Command(os.Args[0], "-test.run=TestUIIdentityHelper", "--", "unexpected").CombinedOutput(); err == nil || !strings.Contains(string(out), "accepts no arguments") {
		t.Fatalf("output=%q err=%v", out, err)
	}
}

func TestUIIdentityHelper(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "--" {
		return
	}
	main()
}
