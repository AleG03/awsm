package browser

import (
	"errors"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestFirefoxOnMacUsesApplicationDispatch(t *testing.T) {
	target := "https://example.com/console?Action=login&SigninToken=a+b/c="
	name := "production & admin/東京"
	containerURL := buildContainerURL(name, target)
	cmd, err := firefoxContainerCommand("darwin", containerURL)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/usr/bin/open", "-a", "Firefox", containerURL}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("command = %q, want %q", cmd.Args, want)
	}
	// The URL must be passed as one ordinary argument, never through a shell or
	// --args, which would reintroduce direct Firefox command-line handling.
	values, err := url.ParseQuery(strings.TrimPrefix(cmd.Args[3], "ext+container:"))
	if err != nil {
		t.Fatal(err)
	}
	if values.Get("name") != name || values.Get("url") != target {
		t.Fatalf("container payload changed: %v", values)
	}
}

func TestFirefoxCommandsOnOtherPlatforms(t *testing.T) {
	for _, platform := range []string{"windows", "linux"} {
		t.Run(platform, func(t *testing.T) {
			cmd, err := firefoxContainerCommand(platform, "ext+container:name=test&url=https%3A%2F%2Fexample.com")
			if err != nil {
				t.Fatal(err)
			}
			if len(cmd.Args) != 3 || cmd.Args[1] != "--new-tab" || !strings.HasPrefix(cmd.Args[2], "ext+container:") {
				t.Fatal(cmd.Args)
			}
		})
	}
	if _, err := firefoxContainerCommand("unsupported", "ext+container:test"); err == nil {
		t.Fatal("unsupported platform silently accepted a container request")
	}
}

// A fake launcher exercises process exit handling without opening any browser.
func TestFirefoxLauncherHelper(t *testing.T) {
	if os.Getenv("AWSM_TEST_FIREFOX_LAUNCHER") != "1" {
		return
	}
	os.Exit(23)
}

func TestMacDispatchFailureIsReported(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestFirefoxLauncherHelper$")
	cmd.Env = append(os.Environ(), "AWSM_TEST_FIREFOX_LAUNCHER=1")
	err = launchFirefoxContainer(cmd, "darwin")
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 23 {
		t.Fatalf("dispatch failure was lost: %v", err)
	}
	if !strings.Contains(err.Error(), "could not open Firefox container") {
		t.Fatal(err)
	}
}

func TestMissingFirefoxExecutableIsReported(t *testing.T) {
	cmd := exec.Command(t.TempDir() + "/missing-firefox")
	if err := launchFirefoxContainer(cmd, "linux"); err == nil {
		t.Fatal("launch failure was ignored")
	}
}
