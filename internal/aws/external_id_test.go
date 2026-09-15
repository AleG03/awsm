package aws

import (
	"os"
	"testing"

	"awsm/internal/awsini"
)

func TestRoleProfileRoundTripsExternalID(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)

	err := AddIAMRoleProfile("cross", IAMRoleProfile{
		RoleARN:       "arn:aws:iam::123456789012:role/target",
		SourceProfile: "src",
		ExternalID:    "SHARED-SECRET",
		Region:        "eu-west-1",
	})
	if err != nil {
		t.Fatalf("AddIAMRoleProfile: %v", err)
	}

	cfg, err := awsini.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	section, err := cfg.GetSection("profile cross")
	if err != nil {
		t.Fatalf("profile section missing: %v", err)
	}
	if got := section.Key("external_id").String(); got != "SHARED-SECRET" {
		t.Errorf("external_id = %q", got)
	}

	profiles, err := ListProfilesDetailed()
	if err != nil {
		t.Fatalf("ListProfilesDetailed: %v", err)
	}
	var found bool
	for _, p := range profiles {
		if p.Name == "cross" {
			found = true
			if p.ExternalID != "SHARED-SECRET" {
				t.Errorf("ProfileInfo.ExternalID = %q", p.ExternalID)
			}
		}
	}
	if !found {
		t.Error("profile 'cross' missing from ListProfilesDetailed")
	}
}

func TestUpdateIAMRoleProfileClearsExternalIDWhenEmptied(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)
	if err := os.WriteFile(configPath, []byte(
		"[profile cross]\nrole_arn = arn:aws:iam::1:role/t\nexternal_id = OLD\noutput = json\n"), 0600); err != nil {
		t.Fatal(err)
	}

	err := UpdateIAMRoleProfile("cross", IAMRoleProfile{
		RoleARN: "arn:aws:iam::1:role/t",
		Region:  "eu-west-1",
	})
	if err != nil {
		t.Fatalf("UpdateIAMRoleProfile: %v", err)
	}

	cfg, err := awsini.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	section, _ := cfg.GetSection("profile cross")
	if section.HasKey("external_id") {
		t.Error("external_id should have been removed when cleared")
	}
	// Updating in place must not cost the keys awsm does not manage.
	if got := section.Key("output").String(); got != "json" {
		t.Errorf("unknown key 'output' was lost, got %q", got)
	}
}
