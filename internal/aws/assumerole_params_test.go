package aws

import (
	"os"
	"strings"
	"testing"
)

func TestInspectProfileReadsAssumeRoleKeys(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)
	if err := os.WriteFile(configPath, []byte(`[profile cross]
role_arn = arn:aws:iam::123456789012:role/target
source_profile = src
mfa_serial = arn:aws:iam::123456789012:mfa/u
external_id = SHARED-SECRET
role_session_name = chosen-name
duration_seconds = 7200
`), 0600); err != nil {
		t.Fatal(err)
	}

	pConfig, profileType, err := inspectProfile("cross")
	if err != nil {
		t.Fatalf("inspectProfile: %v", err)
	}
	if profileType != "iam" {
		t.Fatalf("profile type = %q, want iam", profileType)
	}
	if pConfig.ExternalId != "SHARED-SECRET" {
		t.Errorf("ExternalId = %q", pConfig.ExternalId)
	}
	if pConfig.RoleSessionName != "chosen-name" {
		t.Errorf("RoleSessionName = %q", pConfig.RoleSessionName)
	}
	if pConfig.DurationSeconds != 7200 {
		t.Errorf("DurationSeconds = %d, want 7200", pConfig.DurationSeconds)
	}
}

func TestInspectProfileLeavesDurationUnsetWhenAbsent(t *testing.T) {
	// Unset must stay unset so assumeRole omits the parameter, the way the AWS
	// CLI does, rather than capping the session at a hardcoded hour.
	configPath, _ := useTempAWSFiles(t)
	if err := os.WriteFile(configPath, []byte(
		"[profile r]\nrole_arn = arn:aws:iam::123456789012:role/t\nsource_profile = src\n"), 0600); err != nil {
		t.Fatal(err)
	}

	pConfig, _, err := inspectProfile("r")
	if err != nil {
		t.Fatalf("inspectProfile: %v", err)
	}
	if pConfig.DurationSeconds != 0 {
		t.Errorf("DurationSeconds = %d, want 0 (unset)", pConfig.DurationSeconds)
	}
}

func TestInspectProfileRejectsUnusableDuration(t *testing.T) {
	for _, value := range []string{"forever", "-1", "0"} {
		t.Run(value, func(t *testing.T) {
			configPath, _ := useTempAWSFiles(t)
			if err := os.WriteFile(configPath, []byte(
				"[profile r]\nrole_arn = arn:aws:iam::1:role/t\nsource_profile = s\nduration_seconds = "+value+"\n"), 0600); err != nil {
				t.Fatal(err)
			}

			_, _, err := inspectProfile("r")
			if err == nil {
				t.Fatalf("duration_seconds = %q should be an error, not a silent fallback", value)
			}
			if !strings.Contains(err.Error(), "duration_seconds") {
				t.Errorf("error should name the key, got: %v", err)
			}
		})
	}
}

func TestBuildRoleSessionName(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		profile    string
		want       string
		wantPrefix string
	}{
		{
			name:       "a configured name is used as the AWS CLI would",
			configured: "my.session@example.com",
			profile:    "ignored",
			want:       "my.session@example.com",
		},
		{
			name:       "characters STS rejects are replaced",
			configured: "not/valid name!",
			want:       "not-valid-name",
		},
		{
			name:       "runs of replaced characters collapse",
			configured: "a///b",
			want:       "a-b",
		},
		{
			name:       "over-long names are truncated to the STS limit",
			configured: strings.Repeat("x", 100),
			want:       strings.Repeat("x", 64),
		},
		{
			name:       "a name with nothing usable falls back",
			configured: "///",
			want:       "awsm-session",
		},
		{
			name:       "generated names carry the profile",
			profile:    "prod-admin",
			wantPrefix: "awsm-",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildRoleSessionName(tc.configured, tc.profile)
			if tc.want != "" && got != tc.want {
				t.Errorf("BuildRoleSessionName(%q, %q) = %q, want %q", tc.configured, tc.profile, got, tc.want)
			}
			if tc.wantPrefix != "" {
				if !strings.HasPrefix(got, tc.wantPrefix) {
					t.Errorf("got %q, want prefix %q", got, tc.wantPrefix)
				}
				if !strings.Contains(got, tc.profile) {
					t.Errorf("got %q, should name the profile %q", got, tc.profile)
				}
			}
			if len(got) < 2 || len(got) > 64 {
				t.Errorf("got %q (len %d), STS requires 2-64", got, len(got))
			}
			if roleSessionNameUnsafe.MatchString(got) {
				t.Errorf("got %q, which contains characters STS rejects", got)
			}
		})
	}
}

func TestBuildRoleSessionNameTruncatesGeneratedNames(t *testing.T) {
	got := BuildRoleSessionName("", strings.Repeat("p", 200))
	if len(got) > 64 {
		t.Errorf("got %q (len %d), STS requires at most 64", got, len(got))
	}
	if strings.HasSuffix(got, "-") {
		t.Errorf("got %q, truncation left a trailing separator", got)
	}
}
