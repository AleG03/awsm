package util

import (
	"testing"
)

func TestSortBy(t *testing.T) {
	// Test sorting integers
	nums := []int{3, 1, 4, 1, 5}
	SortBy(nums, func(a, b int) bool { return a < b })

	expected := []int{1, 1, 3, 4, 5}
	for i, v := range nums {
		if v != expected[i] {
			t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
		}
	}

	// Test sorting strings
	strs := []string{"banana", "apple", "cherry"}
	SortBy(strs, func(a, b string) bool { return a < b })

	expectedStrs := []string{"apple", "banana", "cherry"}
	for i, v := range strs {
		if v != expectedStrs[i] {
			t.Errorf("Expected %s at index %d, got %s", expectedStrs[i], i, v)
		}
	}
}

func TestColors(t *testing.T) {
	// Test that color variables are initialized
	if InfoColor == nil {
		t.Error("InfoColor should not be nil")
	}
	if SuccessColor == nil {
		t.Error("SuccessColor should not be nil")
	}
	if ErrorColor == nil {
		t.Error("ErrorColor should not be nil")
	}
	if WarnColor == nil {
		t.Error("WarnColor should not be nil")
	}
	if BoldColor == nil {
		t.Error("BoldColor should not be nil")
	}
}
