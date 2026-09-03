package cmd

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	c := &Commands{Version: "test-version"}

	for _, args := range [][]string{{"kb", "--version"}, {"kb", "-v"}} {
		old := os.Stdout
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout = w

		runErr := c.Run(context.Background(), args)

		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		os.Stdout = old

		if runErr != nil {
			t.Fatalf("Run(%v): %v", args, runErr)
		}
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		_ = r.Close()
		if got := buf.String(); got != "kb version test-version\n" {
			t.Fatalf("Run(%v) output = %q, want %q", args, got, "kb version test-version\n")
		}
	}
}

func TestVersionFlagHiddenWithoutVersion(t *testing.T) {
	c := &Commands{}

	err := c.Run(context.Background(), []string{"kb", "--version"})
	if err == nil {
		t.Fatal("expected error for unknown flag --version, got nil")
	}
}
