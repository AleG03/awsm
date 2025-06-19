package main

import (
	"testing"
)

func TestMain(t *testing.T) {
	// This is mostly a wrapper around cmd.Execute(), so we'll just test that the file compiles
	// We can't actually call main() in a test because it would exit the test process

	// Test that the version variables are declared
	if version == "" {
		t.Log("Version is empty, which is expected in tests")
	}

	if commit == "" {
		t.Log("Commit is empty, which is expected in tests")
	}

	if date == "" {
		t.Log("Date is empty, which is expected in tests")
	}
}
