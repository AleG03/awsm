package aws

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"awsm/internal/awsini"
)

// useTempAWSFiles points the config and credentials paths at a temp directory
// for the duration of a test.
func useTempAWSFiles(t *testing.T) (configPath, credentialsPath string) {
	t.Helper()
	dir := t.TempDir()
	configPath = filepath.Join(dir, "config")
	credentialsPath = filepath.Join(dir, "credentials")
	t.Setenv("AWS_CONFIG_FILE", configPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentialsPath)
	// Writing an AWS ini file takes a backup under $HOME. Without this the
	// tests deposit backups of their own fixtures in the developer's real
	// ~/.awsm/backups.
	t.Setenv("HOME", dir)
	InvalidateProfileCache()
	t.Cleanup(InvalidateProfileCache)
	return configPath, credentialsPath
}

func TestAddIAMUserProfileWritesKeysIntoTheConfig(t *testing.T) {
	configPath, credentialsPath := useTempAWSFiles(t)

	if err := AddIAMUserProfile("dev", "AKIAEXAMPLE", "secret", "eu-west-1"); err != nil {
		t.Fatalf("AddIAMUserProfile: %v", err)
	}

	cfg, err := awsini.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	section, err := cfg.GetSection("profile dev")
	if err != nil {
		t.Fatalf("profile section missing: %v", err)
	}
	for key, want := range map[string]string{
		"aws_access_key_id":     "AKIAEXAMPLE",
		"aws_secret_access_key": "secret",
		"region":                "eu-west-1",
	} {
		if got := section.Key(key).String(); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	// The whole point is that the profile lives in one place.
	if _, err := os.Stat(credentialsPath); !os.IsNotExist(err) {
		t.Errorf("credentials file should not have been created, stat error was %v", err)
	}
}

func TestUpdateIAMUserProfileKeepsUnknownKeys(t *testing.T) {
	// Regression guard of the same family as the nested "s3 =" bug: editing a
	// profile used to delete and recreate it, dropping everything awsm does not
	// itself write.
	configPath, _ := useTempAWSFiles(t)
	if err := os.WriteFile(configPath, []byte(
		"[profile dev]\nregion = eu-west-1\naws_access_key_id = OLD\naws_secret_access_key = OLDSECRET\noutput = json\ns3 =\n  addressing_style = path\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := UpdateIAMUserProfile("dev", "NEW", "NEWSECRET", "eu-west-1"); err != nil {
		t.Fatalf("UpdateIAMUserProfile: %v", err)
	}

	cfg, err := awsini.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	section, err := cfg.GetSection("profile dev")
	if err != nil {
		t.Fatalf("profile section missing: %v", err)
	}
	if got := section.Key("aws_access_key_id").String(); got != "NEW" {
		t.Errorf("access key = %q, want NEW", got)
	}
	if got := section.Key("output").String(); got != "json" {
		t.Errorf("unknown key 'output' was lost, got %q", got)
	}
	if nested := section.Key("s3").NestedValues(); len(nested) == 0 {
		t.Error("nested s3 block was lost")
	}
}

func TestUpdateIAMUserProfileRewritesKeysWhereTheyAlreadyLive(t *testing.T) {
	// A profile whose keys are in the credentials file must be updated there.
	// Writing the new pair into the config would leave the old pair in force,
	// because the credentials file takes precedence.
	configPath, credentialsPath := useTempAWSFiles(t)
	if err := os.WriteFile(configPath, []byte("[profile dev]\nregion = eu-west-1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(credentialsPath, []byte(
		"[dev]\naws_access_key_id = OLD\naws_secret_access_key = OLDSECRET\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := UpdateIAMUserProfile("dev", "NEW", "NEWSECRET", "eu-west-1"); err != nil {
		t.Fatalf("UpdateIAMUserProfile: %v", err)
	}

	credCfg, err := awsini.Load(credentialsPath)
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	credSection, err := credCfg.GetSection("dev")
	if err != nil {
		t.Fatalf("credentials section missing: %v", err)
	}
	if got := credSection.Key("aws_access_key_id").String(); got != "NEW" {
		t.Errorf("credentials access key = %q, want NEW", got)
	}

	cfg, err := awsini.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	section, _ := cfg.GetSection("profile dev")
	if got := section.Key("aws_access_key_id").String(); got != "" {
		t.Errorf("config should not have gained a shadowed key, got %q", got)
	}
}

func TestUpdateStaticProfileFindsKeysInEitherFile(t *testing.T) {
	// awsm profile set used to look only in the credentials file, so a profile
	// whose keys are in the config failed with "does not have static
	// credentials".
	for _, tc := range []struct {
		name, config, credentials, wantKey string
	}{
		{
			name:        "keys in the config",
			config:      "[profile dev]\nregion = eu-west-1\naws_access_key_id = FROMCONFIG\naws_secret_access_key = s\n",
			credentials: "",
			wantKey:     "FROMCONFIG",
		},
		{
			name:        "keys in the credentials file",
			config:      "[profile dev]\nregion = eu-west-1\n",
			credentials: "[dev]\naws_access_key_id = FROMCREDS\naws_secret_access_key = s\n",
			wantKey:     "FROMCREDS",
		},
		{
			name:        "both, credentials wins as the AWS CLI does",
			config:      "[profile dev]\naws_access_key_id = FROMCONFIG\naws_secret_access_key = s\n",
			credentials: "[dev]\naws_access_key_id = FROMCREDS\naws_secret_access_key = s\n",
			wantKey:     "FROMCREDS",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configPath, credentialsPath := useTempAWSFiles(t)
			if err := os.WriteFile(configPath, []byte(tc.config), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(credentialsPath, []byte(tc.credentials), 0600); err != nil {
				t.Fatal(err)
			}

			if err := UpdateStaticProfile("dev"); err != nil {
				t.Fatalf("UpdateStaticProfile: %v", err)
			}

			credCfg, err := awsini.Load(credentialsPath)
			if err != nil {
				t.Fatalf("load credentials: %v", err)
			}
			defaultSection, err := credCfg.GetSection("default")
			if err != nil {
				t.Fatalf("default section missing: %v", err)
			}
			if got := defaultSection.Key("aws_access_key_id").String(); got != tc.wantKey {
				t.Errorf("default access key = %q, want %q", got, tc.wantKey)
			}
		})
	}
}

func TestUpdateStaticProfileNamesBothFilesWhenThereAreNoKeys(t *testing.T) {
	configPath, credentialsPath := useTempAWSFiles(t)
	if err := os.WriteFile(configPath, []byte("[profile dev]\nregion = eu-west-1\n"), 0600); err != nil {
		t.Fatal(err)
	}

	err := UpdateStaticProfile("dev")
	if err == nil {
		t.Fatal("expected an error for a profile with no static credentials")
	}
	for _, want := range []string{configPath, credentialsPath} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should name %s, got: %v", want, err)
		}
	}
}

func TestListProfilesDetailedReadsKeysFromEitherFile(t *testing.T) {
	configPath, credentialsPath := useTempAWSFiles(t)
	if err := os.WriteFile(configPath, []byte(
		"[profile in-config]\naws_access_key_id = FROMCONFIG\naws_secret_access_key = s\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(credentialsPath, []byte(
		"[in-creds]\naws_access_key_id = FROMCREDS\naws_secret_access_key = s\n"), 0600); err != nil {
		t.Fatal(err)
	}

	profiles, err := ListProfilesDetailed()
	if err != nil {
		t.Fatalf("ListProfilesDetailed: %v", err)
	}
	keys := map[string]string{}
	for _, p := range profiles {
		keys[p.Name] = p.AccessKey
	}
	if keys["in-config"] != "FROMCONFIG" {
		t.Errorf("config profile access key = %q, want FROMCONFIG", keys["in-config"])
	}
	if keys["in-creds"] != "FROMCREDS" {
		t.Errorf("credentials profile access key = %q, want FROMCREDS", keys["in-creds"])
	}
}
