package aws

import (
	"errors"
	"os"
	"testing"
	"time"

	"awsm/internal/awsini"
)

func TestConditionalClearPreservesAnotherActiveProfile(t *testing.T) {
	for _, requested := range []string{"work", "other", ""} {
		t.Run("requested_"+requested, func(t *testing.T) {
			_, credentials := useTempAWSFiles(t)
			if err := UpdateCredentialsFile(&TempCredentials{AccessKeyId: "FAKE", SecretAccessKey: "fake"}, "eu-west-1", "work"); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(credentials)
			if err != nil {
				t.Fatal(err)
			}
			err = ClearDefaultProfileIfCurrent(requested)
			if requested == "work" {
				if err != nil {
					t.Fatal(err)
				}
				if GetCurrentProfileName() != "" {
					t.Fatal("profile was not cleared")
				}
			} else {
				if !errors.Is(err, ErrActiveProfileChanged) {
					t.Fatalf("expected active profile mismatch, got %v", err)
				}
				after, err := os.ReadFile(credentials)
				if err != nil {
					t.Fatal(err)
				}
				if string(after) != string(before) {
					t.Fatal("cleared another account's credentials")
				}
			}
		})
	}
}

func TestRegionChangeSynchronizesOnlyTheMatchingActiveProfile(t *testing.T) {
	for _, active := range []string{"work", "other"} {
		t.Run(active, func(t *testing.T) {
			config, credentials := useTempAWSFiles(t)
			if err := AddIAMUserProfile("work", "FAKE", "fake", "eu-west-1"); err != nil {
				t.Fatal(err)
			}
			expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
			if err := UpdateCredentialsFile(&TempCredentials{AccessKeyId: "ACTIVE", SecretAccessKey: "fake", SessionToken: "fake", Expires: expiry}, "eu-west-1", active); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(credentials)
			if err != nil {
				t.Fatal(err)
			}
			if err := ChangeProfileRegionAndActive("work", "us-east-1"); err != nil {
				t.Fatal(err)
			}
			cfg, err := awsini.Load(config)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Section("profile work").Key("region").String() != "us-east-1" {
				t.Fatal("named profile region did not change")
			}
			creds, err := awsini.Load(credentials)
			if err != nil {
				t.Fatal(err)
			}
			wantRegion := "eu-west-1"
			if active == "work" {
				wantRegion = "us-east-1"
			}
			if creds.Section("default").Key("region").String() != wantRegion {
				t.Fatal("wrong active region")
			}
			if active == "other" {
				after, _ := os.ReadFile(credentials)
				if string(before) != string(after) {
					t.Fatal("changed another active profile")
				}
			}
			if creds.Section("default").Key("aws_access_key_id").String() != "ACTIVE" {
				t.Fatal("lost active credentials")
			}
			got, ok := ActiveCredentialsExpiry(active)
			if !ok || !got.Equal(expiry) {
				t.Fatal("lost active expiry")
			}
			if GetCurrentProfileName() != active {
				t.Fatal("changed active profile")
			}
		})
	}
}
