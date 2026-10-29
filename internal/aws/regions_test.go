package aws

import (
	"context"
	"testing"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
)

func TestConsoleURLsFollowThePartition(t *testing.T) {
	for _, tc := range []struct {
		region, federation, home string
	}{
		{"eu-west-1", "https://signin.aws.amazon.com/federation", "https://eu-west-1.console.aws.amazon.com/console/home?region=eu-west-1"},
		{"eusc-de-east-1", "https://eusc-de-east-1.signin.amazonaws-eusc.eu/federation", "https://eusc-de-east-1.console.amazonaws-eusc.eu/console/home?region=eusc-de-east-1"},
	} {
		if got := ConsoleFederationURL(tc.region); got != tc.federation {
			t.Errorf("ConsoleFederationURL(%q) = %q, want %q", tc.region, got, tc.federation)
		}
		if got := ConsoleHomeURL(tc.region); got != tc.home {
			t.Errorf("ConsoleHomeURL(%q) = %q, want %q", tc.region, got, tc.home)
		}
	}
	if !IsValidRegion("eusc-de-east-1") {
		t.Error("the European Sovereign Cloud region is rejected")
	}
}

// The SSO login and the role credentials are resolved by the SDK, which has to
// know the aws-eusc partition for it. Releases before that resolve
// eusc-de-east-1 under amazonaws.com, to hosts that do not exist, so the login
// fails on DNS and nothing here says why. This pins the dependency floor.
func TestSDKResolvesSovereignSSOEndpoints(t *testing.T) {
	region := sdkaws.String("eusc-de-east-1")
	oidc, err := ssooidc.NewDefaultEndpointResolverV2().ResolveEndpoint(context.Background(), ssooidc.EndpointParameters{Region: region})
	if err != nil {
		t.Fatal(err)
	}
	if got := oidc.URI.String(); got != "https://oidc.eusc-de-east-1.amazonaws.eu" {
		t.Errorf("ssooidc endpoint = %q", got)
	}
	portal, err := sso.NewDefaultEndpointResolverV2().ResolveEndpoint(context.Background(), sso.EndpointParameters{Region: region})
	if err != nil {
		t.Fatal(err)
	}
	if got := portal.URI.String(); got != "https://portal.sso.eusc-de-east-1.amazonaws.eu" {
		t.Errorf("sso endpoint = %q", got)
	}
}
