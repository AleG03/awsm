package aws

import (
	"os"
	"strings"
	"testing"
	"time"
)

// The gap this closes: nothing recorded when the credentials in the default
// profile stop working. awsm's own cache only ever held the profiles it
// resolves through STS -- never the SSO ones -- so the refresh daemon could not
// tell that an SSO profile was running out, and left it to lapse. Sixteen
// renewals in that daemon's whole history, every one of them a role profile.
func TestTheExpiryOfWhatIsInUseIsRecorded(t *testing.T) {
	useTempAWSFiles(t)
	expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	err := UpdateCredentialsFile(&TempCredentials{
		AccessKeyId:     "AKIAEXAMPLE",
		SecretAccessKey: "secret",
		SessionToken:    "token",
		Expires:         expiry,
	}, "eu-west-1", "work")
	if err != nil {
		t.Fatal(err)
	}

	got, ok := ActiveCredentialsExpiry("work")
	if !ok {
		t.Fatal("no expiry for the profile just written: the daemon would see nothing to renew")
	}
	if !got.Equal(expiry) {
		t.Errorf("expiry = %s, want %s", got, expiry)
	}
}

// TestStaticCredentialsLeaveNoExpiryBehind.
//
// Switching from a profile that expires to one that does not has to take the
// old timestamp with it. Left in place it would read as credentials that ran
// out an hour ago, and the daemon would try to renew keys that never expire.
func TestStaticCredentialsLeaveNoExpiryBehind(t *testing.T) {
	useTempAWSFiles(t)

	if err := UpdateCredentialsFile(&TempCredentials{
		AccessKeyId: "AKIATEMP", SecretAccessKey: "s", SessionToken: "t",
		Expires: time.Now().Add(time.Hour),
	}, "eu-west-1", "temporary"); err != nil {
		t.Fatal(err)
	}
	// Now a static profile: no session token, no expiry.
	if err := UpdateCredentialsFile(&TempCredentials{
		AccessKeyId: "AKIASTATIC", SecretAccessKey: "s",
	}, "eu-west-1", "static"); err != nil {
		t.Fatal(err)
	}

	if _, ok := ActiveCredentialsExpiry("static"); ok {
		t.Error("kept an expiry from the profile before: static keys do not expire")
	}
}

// TestCredentialsWrittenBeforeThisExisted: an upgrade must not leave the daemon
// blind until the next switch, so a profile with no recorded expiry still
// answers from whatever awsm had resolved for it.
func TestCredentialsWrittenBeforeThisExisted(t *testing.T) {
	useTempAWSFiles(t)
	expiry := time.Now().Add(30 * time.Minute).UTC().Truncate(time.Second)

	// The default profile as an older awsm left it: no expiry recorded.
	if err := UpdateCredentialsFile(&TempCredentials{
		AccessKeyId: "AKIA", SecretAccessKey: "s", SessionToken: "t",
	}, "eu-west-1", "role"); err != nil {
		t.Fatal(err)
	}
	// But awsm's own cache has one, as it would for a role profile.
	setCachedCreds("role", &TempCredentials{
		AccessKeyId: "AKIA", SecretAccessKey: "s", SessionToken: "t", Expires: expiry,
	})

	got, ok := ActiveCredentialsExpiry("role")
	if !ok {
		t.Fatal("fell through to nothing; renewal would stop until the next switch")
	}
	if !got.Equal(expiry) {
		t.Errorf("expiry = %s, want the cached %s", got, expiry)
	}
}

func TestClearingTheProfileClearsTheExpiry(t *testing.T) {
	useTempAWSFiles(t)

	if err := UpdateCredentialsFile(&TempCredentials{
		AccessKeyId: "AKIA", SecretAccessKey: "s", SessionToken: "t",
		Expires: time.Now().Add(time.Hour),
	}, "eu-west-1", "work"); err != nil {
		t.Fatal(err)
	}
	if err := ClearDefaultProfile(); err != nil {
		t.Fatal(err)
	}

	credentialsPath, _ := GetAWSCredentialsPath()
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), defaultExpiresKey) {
		t.Errorf("the expiry outlived the credentials it described:\n%s", data)
	}
}

// TestTheExpiryIsInvisibleToTheAwsCli.
//
// The key is written into a file the AWS CLI parses. It begins with '#' so that
// Python's RawConfigParser reads the whole line as a comment -- the same trick
// as '# source_profile', which has been in this file all along. A key it could
// actually see would be an unknown setting in every AWS call the user makes.
func TestTheExpiryIsInvisibleToTheAwsCli(t *testing.T) {
	if !strings.HasPrefix(defaultExpiresKey, "#") {
		t.Fatalf("defaultExpiresKey = %q: a name the AWS CLI can see would become an unknown setting", defaultExpiresKey)
	}
}

// TestChangingTheRegionKeepsTheExpiry.
//
// SetRegion leaves the credentials themselves alone, so the record of when they
// run out has to survive with them. It would not survive on its own: ini.v1
// drops a '#'-prefixed key when it reads the file, so anything of that kind is
// lost unless the write puts it back -- which is why '# source_profile' has
// always been re-written there by hand.
//
// Losing it would leave the refresh daemon blind to that profile until the next
// switch, and the symptom -- credentials quietly lapsing after a region change
// -- points nowhere near this function.
func TestChangingTheRegionKeepsTheExpiry(t *testing.T) {
	useTempAWSFiles(t)
	expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	if err := UpdateCredentialsFile(&TempCredentials{
		AccessKeyId: "AKIA", SecretAccessKey: "s", SessionToken: "t", Expires: expiry,
	}, "eu-west-1", "work"); err != nil {
		t.Fatal(err)
	}

	if err := SetRegion("us-east-1"); err != nil {
		t.Fatal(err)
	}

	got, ok := ActiveCredentialsExpiry("work")
	if !ok {
		t.Fatal("the expiry did not survive a region change; the daemon would stop renewing this profile")
	}
	if !got.Equal(expiry) {
		t.Errorf("expiry = %s, want %s", got, expiry)
	}
	if name := GetCurrentProfileName(); name != "work" {
		t.Errorf("active profile = %q, want it preserved too", name)
	}
}
