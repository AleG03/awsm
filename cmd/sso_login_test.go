package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"awsm/internal/aws"
)

func TestSSOLoginRenewsOnlyTheUnchangedActiveProfile(t *testing.T) {
	for _, scenario := range []string{"renew", "other session", "switch during login", "clear during login", "switch during resolution", "login fails", "resolution fails"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", dir)
			config := filepath.Join(dir, "config")
			credentials := filepath.Join(dir, "credentials")
			t.Setenv("AWS_CONFIG_FILE", config)
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentials)
			aws.InvalidateProfileCache()
			t.Cleanup(aws.InvalidateProfileCache)
			if err := os.WriteFile(config, []byte("[profile work]\nsso_session = corp\nregion = eu-west-1\n"), 0600); err != nil {
				t.Fatal(err)
			}
			old := &aws.TempCredentials{AccessKeyId: "OLD", SecretAccessKey: "fake", SessionToken: "fake", Expires: time.Now().Add(-time.Hour)}
			fresh := &aws.TempCredentials{AccessKeyId: "NEW", SecretAccessKey: "fake", SessionToken: "fake", Expires: time.Now().Add(time.Hour)}
			if err := aws.UpdateCredentialsFile(old, "eu-west-1", "work"); err != nil {
				t.Fatal(err)
			}
			loginDone := false
			resolved := false
			login := func(context.Context, string) error {
				loginDone = true
				switch scenario {
				case "switch during login":
					return aws.UpdateCredentialsFile(fresh, "us-east-1", "other")
				case "clear during login":
					return aws.ClearDefaultProfile()
				case "login fails":
					return errors.New("login failed")
				}
				return nil
			}
			resolve := func(profile string) (*aws.TempCredentials, bool, error) {
				if !loginDone || profile != "work" {
					t.Fatal("resolved before login or wrong profile")
				}
				resolved = true
				if scenario == "resolution fails" {
					return nil, false, errors.New("offline")
				}
				if scenario == "switch during resolution" {
					if err := aws.UpdateCredentialsFile(fresh, "us-east-1", "other"); err != nil {
						t.Fatal(err)
					}
				}
				return fresh, false, nil
			}
			session := "corp"
			if scenario == "other session" {
				session = "different"
			}
			err := loginAndRefreshActive(t.Context(), session, login, resolve)
			if scenario == "login fails" || scenario == "resolution fails" {
				if err == nil {
					t.Fatal("lost failure")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			wantProfile := "work"
			if scenario == "switch during login" || scenario == "switch during resolution" {
				wantProfile = "other"
			}
			if scenario == "clear during login" {
				wantProfile = ""
			}
			if got := aws.GetCurrentProfileName(); got != wantProfile {
				t.Fatalf("active profile=%q want %q", got, wantProfile)
			}
			if scenario == "renew" {
				expiry, ok := aws.ActiveCredentialsExpiry("work")
				if !ok || !expiry.After(time.Now()) {
					t.Fatal("login left active credentials expired")
				}
			}
			if scenario == "other session" || scenario == "switch during login" || scenario == "clear during login" || scenario == "login fails" {
				if resolved {
					t.Fatal("resolved a profile no longer targeted by login")
				}
			}
			if scenario == "resolution fails" || scenario == "login fails" {
				expiry, _ := aws.ActiveCredentialsExpiry("work")
				if expiry.After(time.Now()) {
					t.Fatal("failure marked credentials as renewed")
				}
			}
		})
	}
}
