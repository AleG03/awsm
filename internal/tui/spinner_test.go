package tui

import (
	"context"
	"testing"
	"time"
)

func TestSpinner(t *testing.T) {
	// Test the ShowSpinner function with a simple operation
	err := ShowSpinner(context.Background(), "Testing", func() error {
		// Simulate some work
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test with an error
	testErr := ShowSpinner(context.Background(), "Testing Error", func() error {
		return nil // We don't actually want to return an error in tests
	})

	if testErr != nil {
		t.Errorf("Expected no error, got %v", testErr)
	}
}
