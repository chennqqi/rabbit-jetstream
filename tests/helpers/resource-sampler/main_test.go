package main

import "testing"

func TestParseNodes(t *testing.T) {
	nodes, err := parseNodes("nats-1=http://one,nats-2=http://two/,nats-3=http://three")
	if err != nil || len(nodes) != 3 || nodes["nats-2"] != "http://two" {
		t.Fatalf("nodes=%v err=%v", nodes, err)
	}
	for _, invalid := range []string{"", "a=x", "a=x,a=y,b=z", "a=x,b=y,c="} {
		if _, err := parseNodes(invalid); err == nil {
			t.Fatalf("accepted %q", invalid)
		}
	}
}

func TestFreeBytes(t *testing.T) {
	if value, err := freeBytes(t.TempDir()); err != nil || value == 0 {
		t.Fatalf("freeBytes=%d err=%v", value, err)
	}
}
