package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"awsm/internal/aws"
	"awsm/internal/daemon"
	"awsm/internal/tui"

	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Keep the active profile's credentials from expiring",
	Long: `Renews the credentials of the active profile before they expire.

Nothing stays resident: the system scheduler runs awsm briefly once a minute,
it checks how much life the credentials have left and exits. A cycle that finds
nothing to do costs a few file reads and no network traffic.

Credentials are renewed once less than ten minutes remain. That margin is a
retry budget, not a safety cushion: everything under it is time available to
recover from a failed attempt before anything actually expires.

Profiles needing an MFA code, and SSO sessions that can no longer be refreshed,
cannot be renewed without you. Those are detected before any network call and
reported once, rather than retried every minute.`,
}

var (
	daemonAutoLogin bool
	daemonInterval  time.Duration
)

var daemonEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Start renewing credentials, and keep doing it at every login",
	RunE: func(cmd *cobra.Command, args []string) error {
		if daemonInterval < 10*time.Second {
			return fmt.Errorf("interval %s is too short; the renewal threshold is %s, so anything near it removes the retry budget entirely",
				daemonInterval, daemon.CredentialsThreshold)
		}
		if daemonInterval >= daemon.CredentialsThreshold {
			return fmt.Errorf("interval %s is not shorter than the %s renewal threshold, so a cycle could land after the credentials have already expired",
				daemonInterval, daemon.CredentialsThreshold)
		}

		if err := daemon.Install(daemon.InstallConfig{
			Interval:  daemonInterval,
			AutoLogin: daemonAutoLogin,
		}); err != nil {
			return err
		}

		tui.PrintSuccess(fmt.Sprintf("Credential renewal enabled, checking every %s.", daemonInterval))
		if daemonAutoLogin {
			tui.PrintMuted("An expired SSO session will open the login page automatically.")
		} else {
			tui.PrintMuted("An expired SSO session will be reported, not opened; use --auto-login to change that.")
		}
		return nil
	},
}

var daemonDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Stop renewing credentials and remove the scheduled job",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := daemon.Uninstall(); err != nil {
			return err
		}
		tui.PrintSuccess("Credential renewal disabled.")
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether renewal is running and what it last did",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, err := daemon.Status()
		if err != nil {
			return err
		}

		tui.PrintHeader("Credential renewal")
		switch {
		case status.Installed && status.Loaded:
			tui.PrintKeyValue("State", "enabled")
		case status.Installed:
			tui.PrintKeyValue("State", "installed but not loaded")
		default:
			tui.PrintKeyValue("State", "disabled")
			tui.PrintMuted("Enable it with: awsm daemon enable")
			return nil
		}
		if status.Interval > 0 {
			tui.PrintKeyValue("Every", status.Interval.String())
		}
		tui.PrintKeyValue("Job", status.UnitPath)

		state := daemon.LoadState()
		if !state.LastRun.IsZero() {
			tui.PrintKeyValue("Last run", fmt.Sprintf("%s (%s ago)",
				state.LastRun.Format(time.RFC3339), time.Since(state.LastRun).Round(time.Second)))
			tui.PrintKeyValue("Last result", state.LastAction)
			if state.LastReason != "" {
				tui.PrintKeyValue("Reason", state.LastReason)
			}
			if state.LastError != "" {
				tui.PrintError(state.LastError)
			}
		} else {
			tui.PrintMuted("It has not run yet.")
		}

		if profile := aws.GetCurrentProfileName(); profile != "" {
			if expiry, ok := aws.ActiveCredentialsExpiry(profile); ok {
				tui.PrintKeyValue("Active profile", fmt.Sprintf("%s, credentials expire %s",
					profile, expiry.Format(time.RFC3339)))
			}
		}
		return nil
	},
}

var daemonLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show recent renewal activity",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := daemon.LogPath()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				tui.PrintMuted("No log yet: nothing has needed renewing.")
				return nil
			}
			return err
		}

		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		if len(lines) > 50 {
			lines = lines[len(lines)-50:]
		}
		for _, line := range lines {
			fmt.Println(line)
		}
		return nil
	},
}

// daemonTickCmd is what the scheduler invokes. Hidden because running it by
// hand is only useful when working on awsm itself.
var daemonTickCmd = &cobra.Command{
	Use:    "tick",
	Short:  "Run one renewal cycle",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		decision := daemon.Tick(daemon.Options{AutoLogin: daemonAutoLogin})
		// Printed for anyone running it by hand; under a scheduler nothing
		// reads stdout.
		fmt.Printf("%s: %s\n", decision.Action, decision.Reason)
		return nil
	},
}

func init() {
	daemonEnableCmd.Flags().BoolVar(&daemonAutoLogin, "auto-login", false,
		"open the SSO login page when the session can no longer be refreshed")
	daemonEnableCmd.Flags().DurationVar(&daemonInterval, "interval", daemon.DefaultInterval,
		"how often to check the credentials")
	daemonTickCmd.Flags().BoolVar(&daemonAutoLogin, "auto-login", false,
		"open the SSO login page when the session can no longer be refreshed")

	daemonCmd.AddCommand(daemonEnableCmd, daemonDisableCmd, daemonStatusCmd, daemonLogsCmd, daemonTickCmd)
	rootCmd.AddCommand(daemonCmd)
}
