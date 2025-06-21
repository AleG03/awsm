package tui

import (
	"testing"
)

func TestStyles(t *testing.T) {
	// Test that styles are initialized by checking if they can render content
	if InfoStyle.Render("test") == "" {
		t.Error("InfoStyle should be able to render content")
	}
	if SuccessStyle.Render("test") == "" {
		t.Error("SuccessStyle should be able to render content")
	}
	if ErrorStyle.Render("test") == "" {
		t.Error("ErrorStyle should be able to render content")
	}
	if WarningStyle.Render("test") == "" {
		t.Error("WarningStyle should be able to render content")
	}
	if SpinnerStyle.Render("test") == "" {
		t.Error("SpinnerStyle should be able to render content")
	}
}

func TestSpinnerModel(t *testing.T) {
	model := NewSpinner("test message")

	if model.message != "test message" {
		t.Errorf("Expected 'test message', got '%s'", model.message)
	}

	if model.quitting {
		t.Error("Expected spinner not to be quitting initially")
	}

	if model.err != nil {
		t.Errorf("Expected no error initially, got %v", model.err)
	}
}
