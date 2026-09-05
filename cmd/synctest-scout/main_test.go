package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPatchRequiresReviewAndDoesNotEmitPartialDiff(t *testing.T) {
	var out, stderr bytes.Buffer
	err := run([]string{"patch", "-file", "unknown_test.go", "-test", "TestExample"}, &out, &stderr)
	if err == nil || !strings.Contains(err.Error(), "-reviewed") {
		t.Fatalf("got %v", err)
	}
	if out.Len() != 0 {
		t.Fatal("partial patch emitted")
	}
}
