package aws

import (
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// profileCache caches the profile list to avoid repeated AWS config parsing
var (
	profileCache      []string
	profileCacheMutex sync.RWMutex
	profileCacheValid bool
)

// FuzzyMatch checks if the input matches the target string in a fuzzy way.
// It matches if all characters in input appear in order in the target string.
// The matching is case-insensitive.
func FuzzyMatch(target, input string) bool {
	if input == "" {
		return true
	}

	// Early termination: if input is longer than target, it can't match
	if len(input) > len(target) {
		return false
	}

	// Use byte-level comparison for better performance
	// Convert to lowercase inline to avoid string allocation
	inputIndex := 0
	inputLen := len(input)

	for i := 0; i < len(target) && inputIndex < inputLen; i++ {
		targetChar := target[i]
		inputChar := input[inputIndex]

		// Convert to lowercase inline (more efficient than strings.ToLower)
		if targetChar >= 'A' && targetChar <= 'Z' {
			targetChar += 32
		}
		if inputChar >= 'A' && inputChar <= 'Z' {
			inputChar += 32
		}

		if targetChar == inputChar {
			inputIndex++
		}
	}

	return inputIndex == inputLen
}

func FuzzyMatchUnicode(target, input string) bool {
	if input == "" {
		return true
	}

	if len(input) > len(target) {
		return false
	}

	targetRunes := []rune(strings.ToLower(target))
	inputRunes := []rune(strings.ToLower(input))

	inputIndex := 0
	for i := 0; i < len(targetRunes) && inputIndex < len(inputRunes); i++ {
		if targetRunes[i] == inputRunes[inputIndex] {
			inputIndex++
		}
	}

	return inputIndex == len(inputRunes)
}

// getCachedProfiles returns cached profiles or fetches them if cache is invalid
func getCachedProfiles() ([]string, error) {
	profileCacheMutex.RLock()
	if profileCacheValid && profileCache != nil {
		defer profileCacheMutex.RUnlock()
		return profileCache, nil
	}
	profileCacheMutex.RUnlock()

	// Cache miss or invalid, fetch profiles
	profileCacheMutex.Lock()
	defer profileCacheMutex.Unlock()

	// Double-check in case another goroutine updated the cache
	if profileCacheValid && profileCache != nil {
		return profileCache, nil
	}

	profiles, err := ListProfiles()
	if err != nil {
		return nil, err
	}

	profileCache = profiles
	profileCacheValid = true
	return profiles, nil
}

func InvalidateProfileCache() {
	profileCacheMutex.Lock()
	defer profileCacheMutex.Unlock()
	profileCacheValid = false
	profileCache = nil
}

func CompleteProfiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	profiles, err := getCachedProfiles()
	if err != nil {
		// If we can't list profiles, return no completions but don't error
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Pre-allocate slice with estimated capacity to reduce allocations
	matches := make([]string, 0, len(profiles)/4)

	for _, profile := range profiles {
		if FuzzyMatch(profile, toComplete) {
			matches = append(matches, profile)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// CompleteProfilesFiltered provides profile completion with fuzzy matching and filtering.
// The filter function allows you to exclude certain profiles (e.g., sso-session profiles).
func CompleteProfilesFiltered(filter func(string) bool) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		profiles, err := getCachedProfiles()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Pre-allocate slice with estimated capacity
		matches := make([]string, 0, len(profiles)/4)

		for _, profile := range profiles {
			// Apply filter first (cheaper operation), then fuzzy matching
			if filter(profile) && FuzzyMatch(profile, toComplete) {
				matches = append(matches, profile)
			}
		}

		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteProfilesFast provides the fastest profile completion using different algorithms
// based on input characteristics for optimal performance across different scenarios
func CompleteProfilesFast(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	profiles, err := getCachedProfiles()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Choose algorithm based on input characteristics
	if toComplete == "" {
		// Return all profiles for empty input (most common case)
		return profiles, cobra.ShellCompDirectiveNoFileComp
	}

	matches := make([]string, 0, len(profiles)/4)

	if len(toComplete) == 1 {
		// Single character: use simple prefix matching (faster for single chars)
		char := toComplete[0]
		if char >= 'A' && char <= 'Z' {
			char += 32 // Convert to lowercase
		}

		for _, profile := range profiles {
			if len(profile) > 0 {
				firstChar := profile[0]
				if firstChar >= 'A' && firstChar <= 'Z' {
					firstChar += 32
				}
				if firstChar == char {
					matches = append(matches, profile)
				}
			}
		}
	} else {
		// Multi-character: use fuzzy matching
		for _, profile := range profiles {
			if FuzzyMatch(profile, toComplete) {
				matches = append(matches, profile)
			}
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}
