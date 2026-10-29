package cmd

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"awsm/internal/aws"
	"github.com/spf13/cobra"
)

func prepareConsoleTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_REGION", "eu-west-1")
	t.Setenv("AWS_PROFILE", "different-profile")
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("[profile work & admin/東京]\nsso_session = corp\nregion = eu-west-1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	aws.InvalidateProfileCache()
	t.Cleanup(aws.InvalidateProfileCache)
	oldProfile, oldChrome, oldFirefox, oldZen, oldNoOpen := profileName, chromeProfile, useFirefox, useZen, dontOpenBrowser
	t.Cleanup(func() {
		profileName, chromeProfile, useFirefox, useZen, dontOpenBrowser = oldProfile, oldChrome, oldFirefox, oldZen, oldNoOpen
	})
	profileName, chromeProfile, useFirefox, useZen, dontOpenBrowser = "work & admin/東京", "", false, false, false
}

func TestConsoleUsesTheSameDestinationForSSOAndConsole(t *testing.T) {
	for _, destination := range []string{"firefox", "zen", "chrome", "default", "no-open"} {
		for _, scenario := range []string{"expired", "valid", "open fails", "cancelled"} {
			t.Run(destination+"/"+scenario, func(t *testing.T) {
				prepareConsoleTest(t)
				useFirefox, useZen = destination == "firefox", destination == "zen"
				if destination == "chrome" {
					chromeProfile = "work-chrome"
				}
				dontOpenBrowser = destination == "no-open"
				// --no-open must win even when a container was selected.
				if dontOpenBrowser {
					useFirefox = true
				}
				wantChrome, wantFirefox, wantZen := chromeProfile, "", ""
				if useFirefox {
					wantFirefox = profileName
				}
				if useZen {
					wantZen = profileName
				}
				strict := destination != "default" && destination != "no-open"
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				command := &cobra.Command{}
				command.SetContext(ctx)
				var stdout strings.Builder
				command.SetOut(&stdout)
				const ssoURL = "https://example.awsapps.com/start/#/device?user_code=ABCD-EFGH"
				openErr := errors.New("selected browser unavailable")
				var opened []string
				resolutions, logins, federationCalls := 0, 0, 0
				deps := consoleDependencies{
					credentials: func(profile string, _ ...string) (*aws.TempCredentials, bool, error) {
						if profile != "work & admin/東京" {
							t.Fatalf("wrong profile: %q", profile)
						}
						resolutions++
						if resolutions == 1 && scenario != "valid" {
							return nil, false, aws.ErrSsoSessionExpired
						}
						return &aws.TempCredentials{AccessKeyId: "FAKE", SecretAccessKey: "fake", SessionToken: "fake", Expires: time.Now().Add(time.Hour)}, false, nil
					},
					ssoSession: func(profile string) (string, error) {
						if profile != profileName {
							t.Fatalf("wrong login profile: %q", profile)
						}
						return "corp", nil
					},
					openURL: func(target, chrome, firefox, zen string) error {
						if chrome != wantChrome || firefox != wantFirefox || zen != wantZen {
							t.Fatalf("destination = %q/%q/%q, want %q/%q/%q", chrome, firefox, zen, wantChrome, wantFirefox, wantZen)
						}
						opened = append(opened, target)
						if scenario == "open fails" && target == ssoURL {
							return openErr
						}
						return nil
					},
					login: func(loginCtx context.Context, session string, opts aws.SSOLoginOptions) error {
						logins++
						if loginCtx != ctx || session != "corp" || opts.FailOnOpenError != strict {
							t.Fatalf("login lost context, session or opening policy")
						}
						if scenario == "cancelled" {
							cancel()
							return loginCtx.Err()
						}
						if err := opts.OpenURL(ssoURL); err != nil && opts.FailOnOpenError {
							return err
						}
						return nil
					},
					postForm: func(endpoint string, form url.Values) (*http.Response, error) {
						federationCalls++
						if endpoint != "https://signin.aws.amazon.com/federation" || form.Get("Action") != "getSigninToken" {
							t.Fatal("incorrect federation request")
						}
						return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"SigninToken":"test-token"}`))}, nil
					},
				}
				err := runConsole(command, deps)
				if scenario == "cancelled" || (scenario == "open fails" && strict) {
					wantErr := openErr
					if scenario == "cancelled" {
						wantErr = context.Canceled
					}
					if !errors.Is(err, wantErr) {
						t.Fatalf("error = %v, want %v", err, wantErr)
					}
					if resolutions != 1 || federationCalls != 0 {
						t.Fatal("continued after failed or cancelled login")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				wantLogins, wantResolutions := 1, 2
				if scenario == "valid" {
					wantLogins, wantResolutions = 0, 1
				}
				if logins != wantLogins || resolutions != wantResolutions || federationCalls != 1 {
					t.Fatalf("calls: login=%d credentials=%d federation=%d", logins, resolutions, federationCalls)
				}
				if dontOpenBrowser {
					if len(opened) != 0 || !strings.Contains(stdout.String(), "SigninToken=test-token") {
						t.Fatal("--no-open opened a browser or lost the console URL")
					}
					return
				}
				if len(opened) != 1+wantLogins {
					t.Fatalf("opened URLs = %v", opened)
				}
				if wantLogins == 1 && opened[0] != ssoURL {
					t.Fatal("SSO verification URL changed")
				}
				consoleURL, err := url.Parse(opened[len(opened)-1])
				if err != nil || consoleURL.Query().Get("SigninToken") != "test-token" {
					t.Fatalf("incorrect console URL: %v", opened)
				}
			})
		}
	}
}

func TestConsoleSignsInWithinTheSovereignPartition(t *testing.T) {
	prepareConsoleTest(t)
	t.Setenv("AWS_REGION", "eusc-de-east-1")
	command := &cobra.Command{}
	command.SetContext(t.Context())
	var opened string
	deps := consoleDependencies{
		credentials: func(string, ...string) (*aws.TempCredentials, bool, error) {
			return &aws.TempCredentials{AccessKeyId: "FAKE", SecretAccessKey: "fake", SessionToken: "fake", Expires: time.Now().Add(time.Hour)}, false, nil
		},
		openURL: func(target, _, _, _ string) error {
			opened = target
			return nil
		},
		postForm: func(endpoint string, form url.Values) (*http.Response, error) {
			if endpoint != "https://eusc-de-east-1.signin.amazonaws-eusc.eu/federation" {
				t.Fatalf("federation endpoint = %q", endpoint)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"SigninToken":"test-token"}`))}, nil
		},
	}
	if err := runConsole(command, deps); err != nil {
		t.Fatal(err)
	}
	consoleURL, err := url.Parse(opened)
	if err != nil {
		t.Fatal(err)
	}
	if consoleURL.Host != "eusc-de-east-1.signin.amazonaws-eusc.eu" ||
		consoleURL.Query().Get("Destination") != "https://eusc-de-east-1.console.amazonaws-eusc.eu/console/home?region=eusc-de-east-1" {
		t.Fatalf("console URL left the sovereign partition: %s", opened)
	}
}

func TestConsoleCommand(t *testing.T) {
	if consoleCmd.Use != "console" {
		t.Errorf("Expected command use 'console', got %s", consoleCmd.Use)
	}

	if consoleCmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	if consoleCmd.Long == "" {
		t.Error("Long description should not be empty")
	}

	if len(consoleCmd.Aliases) == 0 {
		t.Error("Command should have aliases")
	}

	if consoleCmd.RunE == nil {
		t.Error("Command should have a RunE function")
	}

	// Test flag initialization
	foundNoOpen := false
	foundFirefox := false
	foundZen := false
	foundChrome := false

	flagUsages := consoleCmd.Flags().FlagUsages()
	if flagUsages != "" {
		if strings.Contains(flagUsages, "no-open") {
			foundNoOpen = true
		}
		if strings.Contains(flagUsages, "firefox-container") {
			foundFirefox = true
		}
		if strings.Contains(flagUsages, "zen-container") {
			foundZen = true
		}
		if strings.Contains(flagUsages, "chrome-profile") {
			foundChrome = true
		}
	}

	if !foundNoOpen {
		t.Error("Command should have a no-open flag")
	}
	if !foundFirefox {
		t.Error("Command should have a firefox-container flag")
	}
	if !foundZen {
		t.Error("Command should have a zen-container flag")
	}
	if !foundChrome {
		t.Error("Command should have a chrome-profile flag")
	}
}
