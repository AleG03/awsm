package aws

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		target   string
		input    string
		expected bool
	}{
		// Exact matches
		{"besharp-capitals", "besharp-capitals", true},
		{"production", "production", true},

		// Fuzzy matches - characters in order
		{"besharp-capitals", "cap", true},
		{"besharp-capitals", "besharp-cap", true},
		{"besharp-capitals", "bcap", true},
		{"besharp-capitals", "bsc", true},
		{"production-env", "prod", true},
		{"production-env", "pe", true},

		// Case insensitive
		{"BeSharp-Capitals", "cap", true},
		{"PRODUCTION", "prod", true},

		// Non-matches - characters not in order
		{"besharp-capitals", "pac", false},
		{"production", "dorp", false},

		// Empty input should match everything
		{"besharp-capitals", "", true},
		{"production", "", true},

		// Input longer than target (early termination optimization)
		{"prod", "production", false},

		// No match
		{"besharp-capitals", "xyz", false},

		// Edge cases for optimization
		{"a", "ab", false},          // Input longer than target
		{"ABC", "abc", true},        // Case conversion
		{"test-profile", "t", true}, // Single character match
	}

	for _, test := range tests {
		result := FuzzyMatch(test.target, test.input)
		if result != test.expected {
			t.Errorf("FuzzyMatch(%q, %q) = %v, expected %v", test.target, test.input, result, test.expected)
		}
	}
}

func TestFuzzyMatchUnicode(t *testing.T) {
	tests := []struct {
		target   string
		input    string
		expected bool
	}{
		// Unicode characters
		{"café-profile", "café", true},
		{"测试-profile", "测试", true},
		{"Ñoño-Profile", "ñoño", true},

		// Mixed ASCII and Unicode
		{"test-café", "tcafé", true},
		{"test-café", "tc", true},
	}

	for _, test := range tests {
		result := FuzzyMatchUnicode(test.target, test.input)
		if result != test.expected {
			t.Errorf("FuzzyMatchUnicode(%q, %q) = %v, expected %v", test.target, test.input, result, test.expected)
		}
	}
}

func TestCompleteProfilesFiltered(t *testing.T) {
	// Test the filter function creation
	excludeSSOSessions := func(profile string) bool {
		return !strings.HasPrefix(profile, "sso-session")
	}

	completionFunc := CompleteProfilesFiltered(excludeSSOSessions)

	// Test that the function is created correctly
	if completionFunc == nil {
		t.Error("CompleteProfilesFiltered should return a valid completion function")
	}

	// Test the filter logic
	testProfiles := []string{
		"sso-session-test",
		"production-profile",
		"sso-session-another",
		"development-profile",
	}

	var filtered []string
	for _, profile := range testProfiles {
		if excludeSSOSessions(profile) {
			filtered = append(filtered, profile)
		}
	}

	expected := []string{"production-profile", "development-profile"}
	if len(filtered) != len(expected) {
		t.Errorf("Filter should exclude sso-session profiles. Got %v, expected %v", filtered, expected)
	}

	for i, profile := range filtered {
		if profile != expected[i] {
			t.Errorf("Filter result mismatch at index %d. Got %q, expected %q", i, profile, expected[i])
		}
	}
}

func BenchmarkFuzzyMatch(b *testing.B) {
	target := "besharp-capitals-administratoraccess"
	input := "cap"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FuzzyMatch(target, input)
	}
}

func BenchmarkFuzzyMatchLong(b *testing.B) {
	target := "very-long-profile-name-with-many-segments-and-characters"
	input := "vlpnwmsac"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FuzzyMatch(target, input)
	}
}

func BenchmarkFuzzyMatchNoMatch(b *testing.B) {
	target := "besharp-capitals-administratoraccess"
	input := "xyz"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FuzzyMatch(target, input)
	}
}

func TestProfileCaching(t *testing.T) {
	// Invalidate cache first
	InvalidateProfileCache()

	// First call should fetch profiles
	profiles1, err1 := getCachedProfiles()
	if err1 != nil {
		t.Skip("Skipping cache test - no AWS profiles available")
	}

	// Second call should use cache
	profiles2, err2 := getCachedProfiles()
	if err2 != nil {
		t.Errorf("Second call failed: %v", err2)
	}

	// Should return same data
	if len(profiles1) != len(profiles2) {
		t.Errorf("Cache returned different number of profiles: %d vs %d", len(profiles1), len(profiles2))
	}

	// Test cache invalidation
	InvalidateProfileCache()
	profiles3, err3 := getCachedProfiles()
	if err3 != nil {
		t.Errorf("Third call after invalidation failed: %v", err3)
	}

	// Should still return same data (but fetched fresh)
	if len(profiles1) != len(profiles3) {
		t.Errorf("After invalidation returned different number of profiles: %d vs %d", len(profiles1), len(profiles3))
	}
}

func TestCompleteProfilesFast(t *testing.T) {
	// Test empty input
	matches, directive := CompleteProfilesFast(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("Expected NoFileComp directive, got %v", directive)
	}

	// For empty input, should return all profiles (if any available)
	if len(matches) == 0 {
		t.Skip("No profiles available for testing")
	}

	// Test single character input
	singleCharMatches, _ := CompleteProfilesFast(nil, nil, "b")

	// Should have some matches for 'b' (assuming profiles starting with 'b' exist)
	// This is a basic sanity check
	if len(singleCharMatches) > len(matches) {
		t.Errorf("Single char matches (%d) should not exceed total profiles (%d)", len(singleCharMatches), len(matches))
	}
}
