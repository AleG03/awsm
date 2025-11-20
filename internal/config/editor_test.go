package config

import (
	"reflect"
	"testing"
)

func TestExtractProfileConfig(t *testing.T) {
	content := `[profile test]
sso_session = my-session
region = us-east-1
`
	expected := `sso_session = my-session
region = us-east-1`

	result := ExtractProfileConfig(content)
	if result != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, result)
	}
}

func TestRemoveProfileFromConfig(t *testing.T) {
	config := `[profile p1]
region = us-east-1

[profile p2]
region = us-west-2

[profile p3]
region = eu-central-1
`
	expected := `[profile p1]
region = us-east-1

[profile p3]
region = eu-central-1
`

	result := RemoveProfileFromConfig(config, "p2")
	if result != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, result)
	}
}

func TestExtractProfileNamesFromContent(t *testing.T) {
	content := `[profile p1]
region = us-east-1

[profile p2]
region = us-west-2
`
	expected := []string{"p1", "p2"}

	result := ExtractProfileNamesFromContent(content)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestParseExistingProfiles(t *testing.T) {
	config := `[profile p1]
region = us-east-1

[profile p2]
region = us-west-2
`
	expectedProfiles := map[string]bool{
		"p1": true,
		"p2": true,
	}

	profiles, content := ParseExistingProfiles(config)

	if !reflect.DeepEqual(profiles, expectedProfiles) {
		t.Errorf("Expected profiles %v, got %v", expectedProfiles, profiles)
	}

	if len(content) != 2 {
		t.Errorf("Expected 2 profile contents, got %d", len(content))
	}

	if _, ok := content["p1"]; !ok {
		t.Error("Expected content for p1")
	}
}
