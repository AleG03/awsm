package aws

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
)

// fakeOIDC stands in for IAM Identity Center. It records what it was asked,
// because for this flow the contents of the requests are the behaviour worth
// testing: a login that sends the wrong grant types still succeeds.
type fakeOIDC struct {
	registerInputs []*ssooidc.RegisterClientInput
	authInputs     []*ssooidc.StartDeviceAuthorizationInput
	tokenInputs    []*ssooidc.CreateTokenInput

	registerOut  *ssooidc.RegisterClientOutput
	authOut      *ssooidc.StartDeviceAuthorizationOutput
	tokenReplies []tokenReply
}

type tokenReply struct {
	out *ssooidc.CreateTokenOutput
	err error
}

func (f *fakeOIDC) RegisterClient(_ context.Context, in *ssooidc.RegisterClientInput, _ ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error) {
	f.registerInputs = append(f.registerInputs, in)
	return f.registerOut, nil
}

func (f *fakeOIDC) StartDeviceAuthorization(_ context.Context, in *ssooidc.StartDeviceAuthorizationInput, _ ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error) {
	f.authInputs = append(f.authInputs, in)
	return f.authOut, nil
}

func (f *fakeOIDC) CreateToken(_ context.Context, in *ssooidc.CreateTokenInput, _ ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
	f.tokenInputs = append(f.tokenInputs, in)
	if len(f.tokenReplies) == 0 {
		return nil, &types.AuthorizationPendingException{}
	}
	reply := f.tokenReplies[0]
	f.tokenReplies = f.tokenReplies[1:]
	return reply.out, reply.err
}

func str(s string) *string { return &s }

func granted() *ssooidc.CreateTokenOutput {
	return &ssooidc.CreateTokenOutput{
		AccessToken:  str("access-token"),
		RefreshToken: str("refresh-token"),
		ExpiresIn:    3600,
	}
}

// newLogin builds a login whose clock, browser and sleeping are all under the
// test's control, so a flow that really waits minutes finishes instantly.
func newLogin(t *testing.T, client oidcClient, replies ...tokenReply) (*ssoLogin, *[]time.Duration) {
	t.Helper()
	var slept []time.Duration
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

	if fake, ok := client.(*fakeOIDC); ok {
		fake.tokenReplies = replies
		if fake.registerOut == nil {
			fake.registerOut = &ssooidc.RegisterClientOutput{
				ClientId:              str("client-id"),
				ClientSecret:          str("client-secret"),
				ClientSecretExpiresAt: now.Add(90 * 24 * time.Hour).Unix(),
			}
		}
		if fake.authOut == nil {
			fake.authOut = &ssooidc.StartDeviceAuthorizationOutput{
				DeviceCode:              str("device-code"),
				UserCode:                str("ABCD-EFGH"),
				VerificationUriComplete: str("https://example.awsapps.com/start/#/device?user_code=ABCD-EFGH"),
				ExpiresIn:               600,
				Interval:                5,
			}
		}
	}

	return &ssoLogin{
		client: client,
		session: SSOSessionInfo{
			Name:     "example-sso",
			StartURL: "https://example.awsapps.com/start",
			Region:   "eu-west-1",
			Scopes:   defaultSSOScope,
		},
		cache: filepath.Join(t.TempDir(), "token.json"),
		out:   io.Discard,
		open:  func(string) error { return nil },
		sleep: func(_ context.Context, d time.Duration) error {
			slept = append(slept, d)
			// The clock only moves when the flow waits, so a poll that never
			// gives up still reaches the device code's expiry.
			now = now.Add(d)
			return nil
		},
		now: func() time.Time { return now },
	}, &slept
}

func readCache(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the cache: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("the cache is not valid JSON: %v", err)
	}
	return fields
}

// TestLoginAsksForARefreshTokenGrant is the test this whole file exists for.
//
// Omitting refresh_token from the registration produces a login that works and
// a session that can never be renewed without a browser: CreateToken simply
// returns no refresh token, RefreshSSOToken then reports a token that "predates
// refresh tokens", and the daemon goes quiet. Nothing else in the flow fails,
// so only an assertion on the request itself can catch it.
func TestLoginAsksForARefreshTokenGrant(t *testing.T) {
	fake := &fakeOIDC{}
	login, _ := newLogin(t, fake, tokenReply{out: granted()})

	if err := login.run(context.Background()); err != nil {
		t.Fatalf("run() = %v, want no error", err)
	}

	if len(fake.registerInputs) != 1 {
		t.Fatalf("RegisterClient called %d times, want 1", len(fake.registerInputs))
	}
	grants := fake.registerInputs[0].GrantTypes
	if !slices.Contains(grants, grantRefreshToken) {
		t.Errorf("GrantTypes = %v, missing %q: the session could never be renewed without a browser", grants, grantRefreshToken)
	}
	if !slices.Contains(grants, grantDeviceCode) {
		t.Errorf("GrantTypes = %v, missing %q", grants, grantDeviceCode)
	}
	if got := fake.registerInputs[0].Scopes; !slices.Equal(got, []string{defaultSSOScope}) {
		t.Errorf("Scopes = %v, want [%s]", got, defaultSSOScope)
	}
}

// TestLoginKeepsTheRefreshTokenItIsGiven guards the other half of the same
// property: asking for the grant is useless if the answer is dropped.
func TestLoginKeepsTheRefreshTokenItIsGiven(t *testing.T) {
	fake := &fakeOIDC{}
	login, _ := newLogin(t, fake, tokenReply{out: granted()})

	if err := login.run(context.Background()); err != nil {
		t.Fatalf("run() = %v, want no error", err)
	}

	fields := readCache(t, login.cache)
	if fields["refreshToken"] != "refresh-token" {
		t.Errorf("refreshToken = %v, want the one CreateToken returned", fields["refreshToken"])
	}
	if fields["accessToken"] != "access-token" {
		t.Errorf("accessToken = %v", fields["accessToken"])
	}
	if fields["registrationExpiresAt"] == nil || fields["registrationExpiresAt"] == "" {
		t.Error("registrationExpiresAt is missing: the next login would register a new client every time")
	}
	if fields["region"] != "eu-west-1" || fields["startUrl"] != "https://example.awsapps.com/start" {
		t.Errorf("region/startUrl = %v/%v", fields["region"], fields["startUrl"])
	}
}

// TestSSOCacheFileForUsesSha1OfTheSessionName pins the file name against an
// oracle computed outside Go (`printf 'example-sso' | shasum -a 1`).
//
// This is not awsm's choice to make: the AWS CLI and the SDK look for exactly
// this name, and writing anywhere else means every other tool stops seeing the
// login.
func TestSSOCacheFileForUsesSha1OfTheSessionName(t *testing.T) {
	t.Setenv("HOME", "/home/test")

	for name, want := range map[string]string{
		"example-sso": "04f8e4a98c7d813fc1022c48992b4fe8aade4d50.json",
		"my-company":  "3a5ac7229cc5fb84a93e5d3487c1c4e4f1375b96.json",
	} {
		got, err := ssoCacheFileFor(name)
		if err != nil {
			t.Fatalf("ssoCacheFileFor(%q) = %v", name, err)
		}
		if filepath.Base(got) != want {
			t.Errorf("ssoCacheFileFor(%q) = %q, want base %q", name, got, want)
		}
		if dir := filepath.Dir(got); dir != "/home/test/.aws/sso/cache" {
			t.Errorf("directory = %q, want the AWS CLI's cache directory", dir)
		}
	}
}

// TestLoginWaitsForApproval: pending is the normal answer while somebody walks
// to their browser, not a failure.
func TestLoginWaitsForApproval(t *testing.T) {
	fake := &fakeOIDC{}
	login, slept := newLogin(t, fake,
		tokenReply{err: &types.AuthorizationPendingException{}},
		tokenReply{err: &types.AuthorizationPendingException{}},
		tokenReply{out: granted()},
	)

	if err := login.run(context.Background()); err != nil {
		t.Fatalf("run() = %v, want no error after two pending replies", err)
	}
	if len(fake.tokenInputs) != 3 {
		t.Errorf("CreateToken called %d times, want 3", len(fake.tokenInputs))
	}
	if got := *fake.tokenInputs[0].GrantType; got != grantDeviceCode {
		t.Errorf("GrantType = %q, want %q", got, grantDeviceCode)
	}
	for _, d := range *slept {
		if d != 5*time.Second {
			t.Errorf("slept %v between polls, want the interval the service asked for", d)
		}
	}
}

// TestLoginBacksOffWhenToldTo: slow_down means the interval was too eager, and
// RFC 8628 section 3.5 says to lengthen it rather than keep the same pace.
func TestLoginBacksOffWhenToldTo(t *testing.T) {
	fake := &fakeOIDC{}
	login, slept := newLogin(t, fake,
		tokenReply{err: &types.SlowDownException{}},
		tokenReply{out: granted()},
	)

	if err := login.run(context.Background()); err != nil {
		t.Fatalf("run() = %v", err)
	}
	if len(*slept) != 2 {
		t.Fatalf("slept %d times, want 2", len(*slept))
	}
	if (*slept)[1] <= (*slept)[0] {
		t.Errorf("interval went %v then %v, want the second to be longer", (*slept)[0], (*slept)[1])
	}
}

// TestLoginGivesUpWhenTheCodeExpires: the flow must end, and say which session
// to try again, rather than poll a dead device code forever.
func TestLoginGivesUpWhenTheCodeExpires(t *testing.T) {
	fake := &fakeOIDC{}
	login, _ := newLogin(t, fake, tokenReply{err: &types.ExpiredTokenException{}})

	err := login.run(context.Background())
	if err == nil {
		t.Fatal("run() = nil, want an error once the device code expired")
	}
	if !strings.Contains(err.Error(), "example-sso") {
		t.Errorf("error = %q, want it to name the session", err)
	}
	if _, statErr := os.Stat(login.cache); statErr == nil {
		t.Error("a failed login wrote a cache file")
	}
}

// TestLoginStopsAtTheDeviceCodeDeadline covers the case the service never
// answers anything but pending: the device code has a life, and the poll must
// respect it instead of running until the process is killed.
func TestLoginStopsAtTheDeviceCodeDeadline(t *testing.T) {
	fake := &fakeOIDC{} // no replies queued: pending forever
	login, _ := newLogin(t, fake)

	if err := login.run(context.Background()); err == nil {
		t.Fatal("run() = nil, want an error once the device code's lifetime ran out")
	}
	// 600s of life at 5s a poll: it must stop near there, not spin.
	if len(fake.tokenInputs) > 130 {
		t.Errorf("CreateToken called %d times, want it bounded by ExpiresIn", len(fake.tokenInputs))
	}
}

// TestLoginReusesAValidRegistration: the AWS CLI registers a client once and
// keeps it until registrationExpiresAt. Registering on every login would leave
// a trail of clients in the Identity Center console.
func TestLoginReusesAValidRegistration(t *testing.T) {
	fake := &fakeOIDC{}
	login, _ := newLogin(t, fake, tokenReply{out: granted()})

	existing := map[string]any{
		"clientId":              "old-client",
		"clientSecret":          "old-secret",
		"registrationExpiresAt": login.now().Add(24 * time.Hour).UTC().Format(ssoCacheTimeLayout),
		"somethingAwsmIgnores":  "keep me",
	}
	raw, _ := json.Marshal(existing)
	if err := os.WriteFile(login.cache, raw, 0600); err != nil {
		t.Fatal(err)
	}

	if err := login.run(context.Background()); err != nil {
		t.Fatalf("run() = %v", err)
	}

	if len(fake.registerInputs) != 0 {
		t.Errorf("RegisterClient called %d times, want 0 with a valid registration cached", len(fake.registerInputs))
	}
	if got := *fake.authInputs[0].ClientId; got != "old-client" {
		t.Errorf("ClientId = %q, want the cached registration", got)
	}

	fields := readCache(t, login.cache)
	if fields["somethingAwsmIgnores"] != "keep me" {
		t.Error("a field awsm does not understand was dropped; the file belongs to the AWS CLI too")
	}
}

// TestLoginRegistersAgainWhenTheRegistrationLapsed is the other side: a stale
// registration must not be reused, or every call fails as an invalid client.
func TestLoginRegistersAgainWhenTheRegistrationLapsed(t *testing.T) {
	fake := &fakeOIDC{}
	login, _ := newLogin(t, fake, tokenReply{out: granted()})

	expired := map[string]any{
		"clientId":              "old-client",
		"clientSecret":          "old-secret",
		"registrationExpiresAt": login.now().Add(-time.Hour).UTC().Format(ssoCacheTimeLayout),
	}
	raw, _ := json.Marshal(expired)
	if err := os.WriteFile(login.cache, raw, 0600); err != nil {
		t.Fatal(err)
	}

	if err := login.run(context.Background()); err != nil {
		t.Fatalf("run() = %v", err)
	}
	if len(fake.registerInputs) != 1 {
		t.Errorf("RegisterClient called %d times, want 1 once the registration lapsed", len(fake.registerInputs))
	}
	if got := readCache(t, login.cache)["clientId"]; got != "client-id" {
		t.Errorf("clientId = %v, want the fresh registration", got)
	}
}

// TestLoginDropsARefreshTokenFromAnOldRegistration.
//
// Re-registering leaves the cache holding a refresh token minted for the client
// that just lapsed. If the new CreateToken returns none of its own, keeping the
// old one produces a cache whose clientId and refreshToken belong to different
// registrations -- which RefreshSSOToken cannot detect, and which the daemon
// would read as the SSO session having ended for good.
func TestLoginDropsARefreshTokenFromAnOldRegistration(t *testing.T) {
	fake := &fakeOIDC{}
	// A grant with no refresh token of its own: the case that exposes a stale
	// one instead of overwriting it.
	withoutRefresh := &ssooidc.CreateTokenOutput{AccessToken: str("access-token"), ExpiresIn: 3600}
	login, _ := newLogin(t, fake, tokenReply{out: withoutRefresh})

	lapsed := map[string]any{
		"clientId":              "old-client",
		"clientSecret":          "old-secret",
		"refreshToken":          "minted-for-the-old-client",
		"registrationExpiresAt": login.now().Add(-time.Hour).UTC().Format(ssoCacheTimeLayout),
	}
	raw, _ := json.Marshal(lapsed)
	if err := os.WriteFile(login.cache, raw, 0600); err != nil {
		t.Fatal(err)
	}

	if err := login.run(context.Background()); err != nil {
		t.Fatalf("run() = %v", err)
	}

	fields := readCache(t, login.cache)
	if fields["clientId"] != "client-id" {
		t.Fatalf("clientId = %v, want the new registration", fields["clientId"])
	}
	if got, present := fields["refreshToken"]; present && got == "minted-for-the-old-client" {
		t.Error("kept a refresh token from the previous registration: it is paired with a clientId that no longer exists, and the daemon would read the failure as an ended SSO session")
	}
}

// TestLoginShowsTheCodeWhenNoBrowserOpens: over SSH, or on a machine with no
// desktop, the printed code is the only way in. A browser that will not open is
// not a reason to fail.
func TestLoginShowsTheCodeWhenNoBrowserOpens(t *testing.T) {
	fake := &fakeOIDC{}
	login, _ := newLogin(t, fake, tokenReply{out: granted()})

	var printed strings.Builder
	login.out = &printed
	login.open = func(string) error { return os.ErrNotExist }

	if err := login.run(context.Background()); err != nil {
		t.Fatalf("run() = %v, want the login to survive a browser that will not open", err)
	}
	if !strings.Contains(printed.String(), "ABCD-EFGH") {
		t.Errorf("output did not show the user code:\n%s", printed.String())
	}
	if !strings.Contains(printed.String(), "https://example.awsapps.com/start/#/device") {
		t.Errorf("output did not show the verification URL:\n%s", printed.String())
	}
}

// TestLoginGivesUpOnAnUnapprovedCode covers what the service really answers.
//
// A code left unapproved for its whole lifetime came back from IAM Identity
// Center as InvalidGrantException, not the documented ExpiredTokenException.
// Treated as an unknown failure it produced a raw HTTP 400 with an empty
// message, which tells the user nothing about what to do next.
func TestLoginGivesUpOnAnUnapprovedCode(t *testing.T) {
	for name, reply := range map[string]error{
		"expired token": &types.ExpiredTokenException{},
		"invalid grant": &types.InvalidGrantException{},
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeOIDC{}
			login, _ := newLogin(t, fake, tokenReply{err: reply})

			err := login.run(context.Background())
			if err == nil {
				t.Fatal("run() = nil, want an error")
			}
			if !strings.Contains(err.Error(), "run it again") {
				t.Errorf("error = %q, want it to say the login must be started again", err)
			}
			if strings.Contains(err.Error(), "StatusCode") {
				t.Errorf("error = %q, want a sentence rather than a raw HTTP failure", err)
			}
		})
	}
}

// TestLoginReportsADenial: refusing in the browser is a decision, not a fault,
// and must not be reported as one.
func TestLoginReportsADenial(t *testing.T) {
	fake := &fakeOIDC{}
	login, _ := newLogin(t, fake, tokenReply{err: &types.AccessDeniedException{}})

	err := login.run(context.Background())
	if err == nil {
		t.Fatal("run() = nil, want an error")
	}
	if !strings.Contains(err.Error(), "denied") {
		t.Errorf("error = %q, want it to say the request was denied", err)
	}
}

func TestReusableRegistration(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour).Format(ssoCacheTimeLayout)

	cases := []struct {
		name   string
		fields map[string]any
		want   bool
	}{
		{"complete and current", map[string]any{"clientId": "a", "clientSecret": "b", "registrationExpiresAt": future}, true},
		{"expired", map[string]any{"clientId": "a", "clientSecret": "b", "registrationExpiresAt": now.Add(-time.Second).Format(ssoCacheTimeLayout)}, false},
		{"no expiry recorded", map[string]any{"clientId": "a", "clientSecret": "b"}, false},
		{"no secret", map[string]any{"clientId": "a", "registrationExpiresAt": future}, false},
		{"unparseable expiry", map[string]any{"clientId": "a", "clientSecret": "b", "registrationExpiresAt": "soon"}, false},
		{"empty", map[string]any{}, false},
		{"rfc3339 with an offset", map[string]any{"clientId": "a", "clientSecret": "b", "registrationExpiresAt": now.Add(time.Hour).Format(time.RFC3339)}, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, got := reusableRegistration(c.fields, now)
			if got != c.want {
				t.Errorf("reusableRegistration() = %v, want %v", got, c.want)
			}
		})
	}
}
