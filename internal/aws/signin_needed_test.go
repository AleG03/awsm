package aws

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"testing"

	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	oidctypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
)

// TestSigningInWouldFixAMissingTokenCache is the bug this was written for.
//
// Switching to an SSO profile whose token cache had been removed failed with a
// raw "no such file or directory" and offered no login, while the same profile
// with an *expired* token signed in by itself. The remedy is the same in both
// cases, and only one of them was being offered it.
func TestSigningInWouldFixAMissingTokenCache(t *testing.T) {
	// Shaped like the SDK's: the os error wrapped in its own sentences.
	missing := fmt.Errorf("failed to refresh cached credentials, failed to read cached SSO token file, %w",
		&fs.PathError{Op: "open", Path: "/home/x/.aws/sso/cache/abc.json", Err: fs.ErrNotExist})

	if !errors.Is(missing, fs.ErrNotExist) {
		t.Fatal("the fixture does not carry fs.ErrNotExist; the test would prove nothing")
	}
	if !signingInWouldFix("sso", missing) {
		t.Error("a missing token cache left the profile with no way forward")
	}
}

// TestSigningInWouldNotFixACredentialProcess: the SSO branch also serves
// credential_process profiles, where a file that does not exist is the
// configured command. Signing in to an Identity Center would not produce it,
// and reporting an expired SSO session would send somebody looking in the
// wrong place entirely.
func TestSigningInWouldNotFixACredentialProcess(t *testing.T) {
	missing := fmt.Errorf("failed to refresh cached credentials, exec: %w",
		&fs.PathError{Op: "fork/exec", Path: "/opt/tools/get-creds", Err: fs.ErrNotExist})

	if signingInWouldFix("credential-process", missing) {
		t.Error("offered an SSO login for a credential_process command that is not installed")
	}
}

func TestSigningInWouldFix(t *testing.T) {
	cases := []struct {
		name        string
		profileType string
		err         error
		want        bool
	}{
		{"expired token", "sso", errors.New("the security token has expired"), true},
		{"expired, plainly", "sso", errors.New("sso session expired"), true},
		{"refresh token rejected", "sso", errors.New("InvalidGrantException: "), true},
		{"missing cache", "sso", fmt.Errorf("read: %w", fs.ErrNotExist), true},
		{"missing cache, os alias", "sso", fmt.Errorf("read: %w", os.ErrNotExist), true},

		// Signing in again fixes none of these, and saying it would turns a
		// readable failure into a wrong instruction.
		{"no network", "sso", errors.New("dial tcp: lookup oidc.eu-west-1.amazonaws.com: no such host"), false},
		{"permissions", "sso", errors.New("AccessDenied: not authorized to perform sts:AssumeRole"), false},
		{"unreadable cache", "sso", fmt.Errorf("read: %w", fs.ErrPermission), false},
		{"process missing", "credential-process", fmt.Errorf("exec: %w", fs.ErrNotExist), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := signingInWouldFix(c.profileType, c.err); got != c.want {
				t.Errorf("signingInWouldFix(%q, %v) = %v, want %v", c.profileType, c.err, got, c.want)
			}
		})
	}
}

// TestGetCredentialsOffersALoginWhenTheCacheIsMissing goes through
// GetCredentialsForProfile rather than the classifier alone.
//
// The classifier being right is not the property that matters: the bug was that
// the caller never consulted one. ErrSsoSessionExpired is what the profile-set
// path watches for to decide whether to sign in, so this asserts the sentinel
// actually comes back out of the function the panel and the CLI both call.
func TestGetCredentialsOffersALoginWhenTheCacheIsMissing(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)
	ssoProfileConfig(t, configPath)
	// Deliberately no token cache: this is a session never signed in to.

	_, _, err := GetCredentialsForProfile("work")
	if err == nil {
		t.Fatal("GetCredentialsForProfile() = nil, want an error with no cached token")
	}
	if !errors.Is(err, ErrSsoSessionExpired) {
		t.Errorf("error = %v\nwant one carrying ErrSsoSessionExpired, which is what makes the caller sign in; "+
			"without it the profile is a dead end and the person is shown a bare file path", err)
	}
}

// TestEveryWayAnSsoSessionGoesBadOffersALogin is the invariant this file
// exists to hold.
//
// An SSO session can become unusable in several ways that look nothing alike to
// the SDK and identical to the person in front of it. Each one not recognised
// is a dead end: a raw error, no login offered, a profile that cannot be
// entered. Three separate ones reached users that way, one after another, each
// found only when somebody hit it. This is the list, so the next one is found
// here instead.
//
// The service errors are constructed rather than provoked: errors.As reaching
// them through the credential provider's wrapping was verified against a real
// rejection from AWS, and pinning the behaviour to the type is what keeps this
// test honest without a network.
func TestEveryWayAnSsoSessionGoesBadOffersALogin(t *testing.T) {
	cases := map[string]error{
		"never signed in, so there is no cached token": fmt.Errorf(
			"failed to refresh cached credentials, failed to read cached SSO token file, %w",
			&fs.PathError{Op: "open", Path: "/home/x/.aws/sso/cache/abc.json", Err: fs.ErrNotExist}),

		// Revoked by an administrator, signed out from the access portal, or
		// ended by the identity provider's own session policy. The token's
		// recorded expiry is still in the future, so nothing about it reads as
		// expired -- this one was a dead end until it was listed.
		"the token is rejected by the service": fmt.Errorf(
			"failed to refresh cached credentials, operation error SSO: GetRoleCredentials, %w",
			&ssotypes.UnauthorizedException{Message: strPtr("Session token not found or invalid")}),

		"the token has run out": fmt.Errorf("operation error SSO OIDC: CreateToken, %w",
			&oidctypes.ExpiredTokenException{}),

		"the refresh token is no longer accepted": fmt.Errorf("operation error SSO OIDC: CreateToken, %w",
			&oidctypes.InvalidGrantException{}),

		// The prose the SDK's own token provider produces, which is not a
		// modelled error and can only be matched by its words.
		"the cached token says it has expired": errors.New(
			"failed to refresh cached credentials, cached SSO token has expired"),
	}

	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			if !signingInWouldFix("sso", err) {
				t.Errorf("no login would be offered for: %v\n\n"+
					"The profile cannot be entered at all, and the person is shown this error "+
					"instead of the browser that would fix it.", err)
			}
		})
	}
}

// TestALoginIsNotOfferedForWhatItWouldNotFix is the other half.
//
// Offering a sign-in for a failure a sign-in does not touch is its own kind of
// wrong: it sends somebody through a browser flow, succeeds, and then fails
// again for the original reason, which is now buried a step further back.
func TestALoginIsNotOfferedForWhatItWouldNotFix(t *testing.T) {
	cases := map[string]struct {
		profileType string
		err         error
	}{
		"the network is down": {"sso", errors.New(
			"dial tcp: lookup oidc.eu-west-1.amazonaws.com: no such host")},
		"the role refuses the caller": {"sso", errors.New(
			"AccessDenied: User is not authorized to perform sts:AssumeRole")},
		"the cache cannot be read": {"sso", fmt.Errorf("read: %w", fs.ErrPermission)},
		"the credential_process command is missing": {"credential-process",
			fmt.Errorf("exec: %w", &fs.PathError{Op: "fork/exec", Path: "/opt/get-creds", Err: fs.ErrNotExist})},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if signingInWouldFix(c.profileType, c.err) {
				t.Errorf("a login was offered for something it cannot fix: %v", c.err)
			}
		})
	}
}
