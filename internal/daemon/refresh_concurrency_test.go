package daemon

import (
	"awsm/internal/aws"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRefreshDoesNotUndoProfileSwitch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	config := `[profile old-account]
region = eu-west-1
credential_process = echo '{"Version":1,"AccessKeyId":"OLD-ACCOUNT","SecretAccessKey":"fake","SessionToken":"fake","Expiration":"2099-01-01T00:00:00Z"}'
`
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if err := aws.UpdateCredentialsFile(&aws.TempCredentials{AccessKeyId: "EXPIRED", SecretAccessKey: "fake", Expires: time.Now().Add(-time.Minute)}, "eu-west-1", "old-account"); err != nil {
		t.Fatal(err)
	}
	s := takeSnapshot(time.Now())
	// Reproduce the interleaving deterministically: the user switches after the
	// daemon's snapshot, before it commits the completed credential resolution.
	if err := aws.UpdateCredentialsFile(&aws.TempCredentials{AccessKeyId: "NEW-ACCOUNT", SecretAccessKey: "fake"}, "eu-west-1", "new-account"); err != nil {
		t.Fatal(err)
	}
	if err := refresh(s, Options{}, &State{}); !errors.Is(err, aws.ErrActiveProfileChanged) {
		t.Fatalf("expected stale renewal rejection, got %v", err)
	}
	if got := aws.GetCurrentProfileName(); got != "new-account" {
		t.Fatalf("refresh switched back to %s", got)
	}
}

func TestTickCommitsCurrentRenewal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	config := `[profile work]
region = eu-west-1
credential_process = echo '{"Version":1,"AccessKeyId":"RENEWED","SecretAccessKey":"fake","SessionToken":"fake","Expiration":"2099-01-01T00:00:00Z"}'
`
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if err := aws.UpdateCredentialsFile(&aws.TempCredentials{AccessKeyId: "EXPIRED", SecretAccessKey: "fake", Expires: time.Now().Add(-time.Minute)}, "eu-west-1", "work"); err != nil {
		t.Fatal(err)
	}
	if d := Tick(Options{}); d.Action != ActionRefresh {
		t.Fatalf("action=%v", d.Action)
	}
	state := LoadState()
	if state.LastError != "" {
		t.Fatal(state.LastError)
	}
	expiry, ok := aws.ActiveCredentialsExpiry("work")
	if !ok || expiry.Before(time.Now()) {
		t.Fatal("tick failed to renew")
	}
}
