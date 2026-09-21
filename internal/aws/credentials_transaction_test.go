package aws

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestRenewalRejectsConcurrentCredentialChanges(t *testing.T) {
	for _, change := range []string{"switch", "clear", "same profile renewal", "region", "unchanged"} {
		t.Run(change, func(t *testing.T) {
			_, cp := useTempAWSFiles(t)
			old := &TempCredentials{AccessKeyId: "OLD", SecretAccessKey: "fake", Expires: time.Now().Add(time.Minute)}
			fresh := &TempCredentials{AccessKeyId: "FRESH", SecretAccessKey: "fake", Expires: time.Now().Add(time.Hour)}
			if err := UpdateCredentialsFile(old, "eu-west-1", "work"); err != nil {
				t.Fatal(err)
			}
			revision, err := ActiveCredentialsRevision()
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "switch":
				err = UpdateCredentialsFile(fresh, "eu-west-1", "other")
			case "clear":
				err = ClearDefaultProfile()
			case "same profile renewal":
				err = UpdateCredentialsFile(fresh, "eu-west-1", "work")
			case "region":
				err = SetRegion("us-east-1")
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(cp)
			if err != nil {
				t.Fatal(err)
			}
			err = UpdateCredentialsFileIfCurrent(fresh, "eu-west-1", "work", revision)
			if change == "unchanged" {
				if err != nil {
					t.Fatal(err)
				}
				after, _ := os.ReadFile(cp)
				if string(after) == string(before) {
					t.Fatal("renewal did not commit")
				}
			} else {
				if !errors.Is(err, ErrActiveProfileChanged) {
					t.Fatalf("expected stale renewal rejection, got %v", err)
				}
				after, _ := os.ReadFile(cp)
				if string(after) != string(before) {
					t.Fatal("stale renewal changed credentials")
				}
			}
		})
	}
}

func TestUnrelatedCredentialEditsKeepSSOExpiry(t *testing.T) {
	for _, op := range []string{"edit", "delete"} {
		t.Run(op, func(t *testing.T) {
			_, cp := useTempAWSFiles(t)
			if err := os.WriteFile(cp, []byte("[other]\naws_access_key_id = FAKE\naws_secret_access_key = fake\n"), 0600); err != nil {
				t.Fatal(err)
			}
			expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
			if err := UpdateCredentialsFile(&TempCredentials{AccessKeyId: "FAKE-SSO", SecretAccessKey: "fake", SessionToken: "fake", Expires: expiry}, "eu-west-1", "sso-work"); err != nil {
				t.Fatal(err)
			}
			var err error
			if op == "edit" {
				err = UpdateIAMUserProfile("other", "NEW-FAKE", "fake", "")
			} else {
				err = DeleteProfile("other")
			}
			if err != nil {
				t.Fatal(err)
			}
			got, ok := ActiveCredentialsExpiry("sso-work")
			if !ok || !got.Equal(expiry) {
				t.Fatal("unrelated edit lost active SSO credential expiry")
			}
			if GetCurrentProfileName() != "sso-work" {
				t.Fatal("unrelated edit lost active profile")
			}
		})
	}
}
