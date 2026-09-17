package aws

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"awsm/internal/browser"
	"awsm/internal/util"

	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
)

// This file performs the SSO login in process, over the OIDC device
// authorization flow, where awsm used to run `aws sso login`.
//
// The AWS CLI is not a dependency worth keeping for this. A GUI and a launchd
// agent inherit a PATH of /usr/bin:/bin:/usr/sbin:/sbin, which is why
// internal/tool exists at all; a login that makes no subprocess has no PATH to
// get wrong. The protocol was already half here, since RefreshSSOToken speaks
// the same API and writes the same cache file.
const (
	// The grant names are the wire values, not Go identifiers: they appear
	// verbatim in the request and are defined by RFC 8628 and RFC 6749.
	grantDeviceCode   = "urn:ietf:params:oauth:grant-type:device_code"
	grantRefreshToken = "refresh_token"

	// defaultSSOScope is what an sso-session without sso_registration_scopes
	// gets. It is the scope the AWS CLI writes when it creates one.
	defaultSSOScope = "sso:account:access"

	// ssoClientName identifies awsm in the IAM Identity Center console, where
	// an administrator reviewing authorized clients will read it.
	ssoClientName = "awsm"

	// defaultPollInterval applies when StartDeviceAuthorization returns no
	// interval of its own.
	defaultPollInterval = 5 * time.Second

	// slowDownIncrement is the backoff RFC 8628 section 3.5 prescribes on a
	// slow_down response: lengthen the interval by five seconds.
	slowDownIncrement = 5 * time.Second

	// ssoCacheTimeLayout is how the AWS CLI writes times in the token cache.
	// Not time.RFC3339: the CLI emits a literal Z rather than an offset.
	ssoCacheTimeLayout = "2006-01-02T15:04:05Z"
)

// oidcClient is the slice of ssooidc the login flow uses.
//
// It exists so the flow can be driven by a fake in tests. The three calls are
// the whole device authorization grant: register once, ask for a code, then
// wait for the person to approve it.
type oidcClient interface {
	RegisterClient(context.Context, *ssooidc.RegisterClientInput, ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error)
	StartDeviceAuthorization(context.Context, *ssooidc.StartDeviceAuthorizationInput, ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error)
	CreateToken(context.Context, *ssooidc.CreateTokenInput, ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error)
}

// ssoLogin carries one login attempt. Everything the flow touches outside its
// own logic -- the clock, the browser, the terminal, sleeping -- arrives as a
// field so a test can supply its own and finish instantly.
type ssoLogin struct {
	client  oidcClient
	session SSOSessionInfo
	cache   string
	out     io.Writer
	open    func(string) error
	sleep   func(context.Context, time.Duration) error
	now     func() time.Time
}

// PerformSSOLogin signs in to an SSO session and caches the resulting token
// where the AWS CLI and the SDK will find it.
func PerformSSOLogin(ssoSession string) error {
	return PerformSSOLoginContext(context.Background(), ssoSession)
}

// PerformSSOLoginContext is PerformSSOLogin with a caller-supplied deadline.
//
// The daemon needs it: waiting minutes for a browser nobody is looking at is
// reasonable for a person at a keyboard and not for a timer under launchd.
func PerformSSOLoginContext(ctx context.Context, ssoSession string) error {
	session, err := lookupSSOSession(ssoSession)
	if err != nil {
		return err
	}
	cache, err := ssoCacheFileFor(session.Name)
	if err != nil {
		return err
	}

	login := &ssoLogin{
		client:  ssooidc.New(ssooidc.Options{Region: session.Region}),
		session: session,
		cache:   cache,
		out:     os.Stderr,
		open:    func(url string) error { return browser.OpenURL(url, "", "", "") },
		sleep:   sleepContext,
		now:     time.Now,
	}
	return login.run(ctx)
}

// lookupSSOSession reads an sso-session block, refusing early on what would
// otherwise fail deep inside an API call.
func lookupSSOSession(name string) (SSOSessionInfo, error) {
	sessions, err := ListSSOSessions()
	if err != nil {
		return SSOSessionInfo{}, fmt.Errorf("could not read the SSO sessions from the AWS config: %w", err)
	}
	for _, s := range sessions {
		if s.Name != name {
			continue
		}
		if s.StartURL == "" {
			return SSOSessionInfo{}, fmt.Errorf("the SSO session '%s' has no sso_start_url", name)
		}
		if s.Region == "" {
			return SSOSessionInfo{}, fmt.Errorf("the SSO session '%s' has no sso_region", name)
		}
		if s.Scopes == "" {
			s.Scopes = defaultSSOScope
		}
		return s, nil
	}
	return SSOSessionInfo{}, fmt.Errorf("no SSO session named '%s' in the AWS config; run 'awsm sso list' to see the configured ones", name)
}

// ssoCacheFileFor is where the token for a session belongs.
//
// The name is the SHA-1 of the session name, which is the AWS CLI's own rule
// and therefore not a choice: write it anywhere else and the CLI, the SDK and
// every other tool reading ~/.aws/sso/cache stop seeing this login. SHA-1 is
// used as a name, not as a defence against anything.
func ssoCacheFileFor(sessionName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find the home directory: %w", err)
	}
	sum := sha1.Sum([]byte(sessionName)) // #nosec G401 -- names a file, per the AWS CLI's format
	return filepath.Join(home, ".aws", "sso", "cache", hex.EncodeToString(sum[:])+".json"), nil
}

// run performs the whole flow and writes the cache only once it has a token.
func (l *ssoLogin) run(ctx context.Context) error {
	fields := l.existingCache()

	clientID, clientSecret, reused := reusableRegistration(fields, l.now())
	if !reused {
		registration, err := l.register(ctx)
		if err != nil {
			return err
		}
		clientID = *registration.ClientId
		clientSecret = *registration.ClientSecret

		// A refresh token belongs to the registration that minted it, so the
		// previous one goes with the previous client.
		//
		// Left in place it would be paired with the new clientId the moment
		// CreateToken returns no refresh token of its own. RefreshSSOToken's
		// guard only checks that the fields are non-empty, so it would sail
		// past, call CreateToken, and be answered InvalidGrantException --
		// which it reads, correctly for its own purposes, as the IdP session
		// having ended. The daemon would then stop trying and report that a
		// browser is needed, when what actually happened is that this login
		// wrote a mismatched pair. Exactly the silence this file exists to
		// prevent, reached without any grant being wrong.
		delete(fields, "refreshToken")

		fields["clientId"] = clientID
		fields["clientSecret"] = clientSecret
		fields["registrationExpiresAt"] = time.Unix(registration.ClientSecretExpiresAt, 0).
			UTC().Format(ssoCacheTimeLayout)
	}

	authorization, err := l.client.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     &clientID,
		ClientSecret: &clientSecret,
		StartUrl:     &l.session.StartURL,
	})
	if err != nil {
		return fmt.Errorf("could not start the SSO login for '%s': %w", l.session.Name, err)
	}

	l.prompt(authorization)

	token, err := l.poll(ctx, clientID, clientSecret, authorization)
	if err != nil {
		return err
	}

	fields["accessToken"] = *token.AccessToken
	fields["expiresAt"] = l.now().UTC().
		Add(time.Duration(token.ExpiresIn) * time.Second).
		Format(ssoCacheTimeLayout)
	fields["region"] = l.session.Region
	fields["startUrl"] = l.session.StartURL
	if token.RefreshToken != nil && *token.RefreshToken != "" {
		fields["refreshToken"] = *token.RefreshToken
	}

	if err := os.MkdirAll(filepath.Dir(l.cache), 0700); err != nil {
		return fmt.Errorf("could not create the SSO cache directory: %w", err)
	}
	if err := writeSSOTokenCache(l.cache, fields); err != nil {
		return err
	}

	util.SuccessColor.Fprintln(l.out, "✔ SSO login successful.")
	return nil
}

// register asks IAM Identity Center for a client of our own.
//
// GrantTypes is the field that decides whether this login can ever be renewed
// without a browser: the list restricts the flows the client may use, so
// omitting refresh_token yields a login that works perfectly and then returns
// no refresh token, leaving RefreshSSOToken -- and with it the daemon -- with
// nothing to renew and no error to report.
func (l *ssoLogin) register(ctx context.Context) (*ssooidc.RegisterClientOutput, error) {
	name := ssoClientName
	clientType := "public"
	out, err := l.client.RegisterClient(ctx, &ssooidc.RegisterClientInput{
		ClientName: &name,
		ClientType: &clientType,
		Scopes:     []string{l.session.Scopes},
		GrantTypes: []string{grantDeviceCode, grantRefreshToken},
	})
	if err != nil {
		return nil, fmt.Errorf("could not register with the SSO service for '%s': %w", l.session.Name, err)
	}
	if out.ClientId == nil || out.ClientSecret == nil {
		return nil, fmt.Errorf("the SSO service registered no client for '%s'", l.session.Name)
	}
	return out, nil
}

// prompt opens the browser and, whether or not that works, prints the URL and
// the code.
//
// Both, always: over SSH or on a machine with no browser the printed code is
// the only way in, and a login that can only succeed with a desktop session is
// a worse tool than the one this replaces.
func (l *ssoLogin) prompt(a *ssooidc.StartDeviceAuthorizationOutput) {
	url := ""
	if a.VerificationUriComplete != nil {
		url = *a.VerificationUriComplete
	} else if a.VerificationUri != nil {
		url = *a.VerificationUri
	}

	util.InfoColor.Fprintf(l.out, "Signing in to SSO session %s\n", util.BoldColor.Sprint(l.session.Name))
	if a.UserCode != nil {
		fmt.Fprintf(l.out, "  code: %s\n", util.BoldColor.Sprint(*a.UserCode))
	}
	if url != "" {
		fmt.Fprintf(l.out, "  open: %s\n", url)
		if err := l.open(url); err != nil {
			util.InfoColor.Fprintln(l.out, "Could not open a browser. Open the address above to continue.")
		}
	}
	util.InfoColor.Fprintln(l.out, "Waiting for approval...")
}

// poll waits for the person to approve the request.
//
// Pending is the expected answer and not an error; slow_down means the interval
// was too eager and must grow. Anything else ends the attempt, because retrying
// a rejected or malformed request only delays the report.
func (l *ssoLogin) poll(ctx context.Context, clientID, clientSecret string, a *ssooidc.StartDeviceAuthorizationOutput) (*ssooidc.CreateTokenOutput, error) {
	interval := time.Duration(a.Interval) * time.Second
	if interval <= 0 {
		interval = defaultPollInterval
	}
	grant := grantDeviceCode
	deadline := l.now().Add(time.Duration(a.ExpiresIn) * time.Second)

	for {
		if err := l.sleep(ctx, interval); err != nil {
			return nil, fmt.Errorf("the SSO login for '%s' was interrupted: %w", l.session.Name, err)
		}

		token, err := l.client.CreateToken(ctx, &ssooidc.CreateTokenInput{
			ClientId:     &clientID,
			ClientSecret: &clientSecret,
			GrantType:    &grant,
			DeviceCode:   a.DeviceCode,
		})
		if err == nil {
			if token.AccessToken == nil || *token.AccessToken == "" {
				return nil, fmt.Errorf("the SSO login for '%s' returned no access token", l.session.Name)
			}
			return token, nil
		}

		var pending *types.AuthorizationPendingException
		var slowDown *types.SlowDownException
		var expired *types.ExpiredTokenException
		var invalidGrant *types.InvalidGrantException
		var denied *types.AccessDeniedException
		switch {
		case errors.As(err, &pending):
			// Nobody has approved it yet, which is the whole point of waiting.
		case errors.As(err, &slowDown):
			interval += slowDownIncrement
		case errors.As(err, &expired), errors.As(err, &invalidGrant):
			// InvalidGrantException is grouped with the documented expiry here
			// because that is what IAM Identity Center actually returns when a
			// device code is left unapproved: observed against a real session,
			// a code abandoned for its full ten minutes came back as
			//
			//	InvalidGrantException: (no message)
			//
			// and never as ExpiredTokenException. RefreshSSOToken reads the
			// same exception the same way. Either one means the code is spent
			// and the only way forward is a new one.
			return nil, fmt.Errorf("the SSO login for '%s' was not approved in time, or the code was already used; run it again", l.session.Name)
		case errors.As(err, &denied):
			return nil, fmt.Errorf("the SSO login for '%s' was denied", l.session.Name)
		default:
			return nil, fmt.Errorf("the SSO login for '%s' failed: %w", l.session.Name, err)
		}

		if !l.now().Before(deadline) {
			return nil, fmt.Errorf("the SSO login for '%s' expired before it was approved; run it again", l.session.Name)
		}
	}
}

// existingCache reads the current token file, keeping every field in it.
//
// The file belongs to the AWS CLI as much as to awsm, and it holds keys neither
// of them needs to understand. A login replaces the parts it knows and leaves
// the rest exactly as it found them.
func (l *ssoLogin) existingCache() map[string]any {
	raw, err := os.ReadFile(l.cache)
	if err != nil {
		return map[string]any{}
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return map[string]any{}
	}
	return fields
}

// reusableRegistration reports whether the cached client registration can be
// used again, which is what the AWS CLI does rather than registering a new
// client on every login.
func reusableRegistration(fields map[string]any, now time.Time) (id, secret string, ok bool) {
	id, _ = fields["clientId"].(string)
	secret, _ = fields["clientSecret"].(string)
	if id == "" || secret == "" {
		return "", "", false
	}
	raw, _ := fields["registrationExpiresAt"].(string)
	if raw == "" {
		return "", "", false
	}
	expires, err := time.Parse(ssoCacheTimeLayout, raw)
	if err != nil {
		// Also accept a full RFC 3339 stamp: the field is written by other
		// tools too, and an unreadable date should cost a registration rather
		// than the login.
		expires, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return "", "", false
		}
	}
	if !now.Before(expires) {
		return "", "", false
	}
	return id, secret, true
}

// sleepContext waits, unless the context ends first.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
