package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExportProtectsSecretsAndRedactsByDefault(t *testing.T) {
	for _, secrets := range []bool{true, false} {
		for _, existing := range []bool{true, false} {
			name := "redacted"
			if secrets {
				name = "secrets"
			}
			if existing {
				name += "-existing"
			}
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				config := filepath.Join(dir, "config")
				t.Setenv("AWS_CONFIG_FILE", config)
				t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
				if err := os.WriteFile(config, []byte("[profile work]\naws_access_key_id = FAKE\naws_secret_access_key = fake-secret\n"), 0600); err != nil {
					t.Fatal(err)
				}
				dest := filepath.Join(dir, "export.json")
				if existing {
					if err := os.WriteFile(dest, []byte("old export"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				old := includeSecrets
				includeSecrets = secrets
				t.Cleanup(func() { includeSecrets = old })
				if err := exportCmd.RunE(exportCmd, []string{dest}); err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(dest)
				if err != nil {
					t.Fatal(err)
				}
				if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
					t.Fatalf("export permissions = %04o", info.Mode().Perm())
				}
				raw, err := os.ReadFile(dest)
				if err != nil {
					t.Fatal(err)
				}
				var data ExportData
				if err := json.Unmarshal(raw, &data); err != nil {
					t.Fatal(err)
				}
				if len(data.Profiles) != 1 {
					t.Fatal("export lost profile")
				}
				if secrets {
					if data.Profiles[0].SecretKey != "fake-secret" || data.ConfigFile == "" {
						t.Fatal("requested secrets missing")
					}
				} else if data.Profiles[0].SecretKey != "" || data.Profiles[0].AccessKey != "" || data.ConfigFile != "" || data.CredentialsFile != "" {
					t.Fatal("default export exposed secrets")
				}
			})
		}
	}
}
