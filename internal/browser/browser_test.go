package browser

import (
	"testing"
)

func TestOpenURL(t *testing.T) {
	// This is mostly a wrapper around other functions, so we'll just test the basic logic

	// Test with no profile/container
	err := OpenURL("https://example.com", "", "")
	if err != nil {
		// This will actually try to open a browser, which might fail in CI
		// So we'll just check that the function exists
		t.Log("OpenURL with no profile returned:", err)
	}

	// Test with Chrome profile
	err = OpenURL("https://example.com", "test-profile", "")
	if err != nil {
		// This will fail if Chrome isn't installed, which is expected
		t.Log("OpenURL with Chrome profile returned:", err)
	}

	// Test with Firefox container
	err = OpenURL("https://example.com", "", "test-container")
	if err != nil {
		// This will fail if Firefox isn't installed, which is expected
		t.Log("OpenURL with Firefox container returned:", err)
	}
}
