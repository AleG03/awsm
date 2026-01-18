package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"awsm/internal/aws"
	"awsm/internal/tui"
	"awsm/internal/util"

	"github.com/spf13/cobra"
)

var (
	portForwarding bool
	remotePort     int
	localPort      int
	remoteHost     string
	configFile     string
)

type ConnectConfig struct {
	InstanceID string `json:"instance_id" yaml:"instance_id"`
	RemoteHost string `json:"remote_host" yaml:"remote_host"`
	RemotePort int    `json:"remote_port" yaml:"remote_port"`
	LocalPort  int    `json:"local_port" yaml:"local_port"`
}

var connectCmd = &cobra.Command{
	Use:   "connect [instance-id]",
	Short: "Connect to an EC2 instance via SSM",
	Long: `Connects to an EC2 instance using AWS Systems Manager (SSM) Session Manager.
It also supports port forwarding to the instance or to a remote host (like RDS) via the instance.

	Examples:
  awsm connect i-0123456789abcdef0                     # Standard shell session
  awsm connect i-0123456789abcdef0 --port-forwarding --remote-port 80 --local-port 8080
  awsm connect i-0123456789abcdef0 -p -r 5432 -H database.internal
  awsm connect --config connect.json`,
	Aliases: []string{"con", "ssm"},
	RunE:    runConnect,
}

func runConnect(cmd *cobra.Command, args []string) error {
	var instanceID string

	// Load config from file if provided
	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		var cfg ConnectConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to parse config file (JSON expected): %w", err)
		}
		instanceID = cfg.InstanceID
		if cfg.RemoteHost != "" {
			remoteHost = cfg.RemoteHost
			portForwarding = true
		}
		if cfg.RemotePort != 0 {
			remotePort = cfg.RemotePort
			portForwarding = true
		}
		if cfg.LocalPort != 0 {
			localPort = cfg.LocalPort
		}
	}

	// Instance ID from args takes precedence
	if len(args) > 0 {
		instanceID = args[0]
	}

	// Get current profile and region
	currentProfile := os.Getenv("AWS_PROFILE")
	if currentProfile == "" {
		currentProfile = aws.GetCurrentProfileName()
	}
	if currentProfile == "" {
		return fmt.Errorf("no AWS profile set. Please run 'awsm profile set <profile-name>' first")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		// Try to get region from profile config
		region, _ = aws.GetProfileRegion(currentProfile)
	}

	// If no instance ID, show selector
	if instanceID == "" {
		instance, err := tui.SelectEC2Instance(currentProfile, region)
		if err != nil {
			return err
		}
		instanceID = instance.InstanceID
	}

	// Prepare aws ssm command
	execArgs := []string{"ssm", "start-session", "--target", instanceID}

	if portForwarding {
		if remotePort == 0 {
			remotePort = 80
		}
		if localPort == 0 {
			localPort = remotePort
		}

		// Check for privileged ports on Unix-like systems
		if localPort < 1024 && os.Geteuid() != 0 {
			util.WarnColor.Fprintf(os.Stderr, "Warning: Local port %d is a privileged port. You might need sudo or to use a higher port (e.g., -l 8080).\n", localPort)
		}

		if remoteHost != "" {
			execArgs = append(execArgs, "--document-name", "AWS-StartPortForwardingSessionToRemoteHost")
			params := map[string][]string{
				"host":            {remoteHost},
				"portNumber":      {strconv.Itoa(remotePort)},
				"localPortNumber": {strconv.Itoa(localPort)},
			}
			paramsJSON, _ := json.Marshal(params)
			execArgs = append(execArgs, "--parameters", string(paramsJSON))

			util.InfoColor.Fprintf(os.Stderr, "Forwarding localhost:%d -> %s:%d via %s\n", localPort, remoteHost, remotePort, instanceID)
		} else {
			execArgs = append(execArgs, "--document-name", "AWS-StartPortForwardingSession")
			params := map[string][]string{
				"portNumber":      {strconv.Itoa(remotePort)},
				"localPortNumber": {strconv.Itoa(localPort)},
			}
			paramsJSON, _ := json.Marshal(params)
			execArgs = append(execArgs, "--parameters", string(paramsJSON))

			util.InfoColor.Fprintf(os.Stderr, "Forwarding localhost:%d -> instance:%d via %s\n", localPort, remotePort, instanceID)
		}
	} else {
		util.InfoColor.Fprintf(os.Stderr, "Connecting to %s...\n", instanceID)
	}

	// Execute aws cli
	ssmCmd := exec.Command("aws", execArgs...)
	ssmCmd.Stdin = os.Stdin
	ssmCmd.Stdout = os.Stdout
	ssmCmd.Stderr = os.Stderr

	// Ensure AWS_PROFILE is set for the subcommand if we inferred it
	if os.Getenv("AWS_PROFILE") == "" && currentProfile != "" {
		ssmCmd.Env = append(os.Environ(), "AWS_PROFILE="+currentProfile)
	}
	if os.Getenv("AWS_REGION") == "" && region != "" {
		if ssmCmd.Env == nil {
			ssmCmd.Env = os.Environ()
		}
		ssmCmd.Env = append(ssmCmd.Env, "AWS_REGION="+region)
	}

	return ssmCmd.Run()
}

func init() {
	connectCmd.Flags().BoolVarP(&portForwarding, "port-forwarding", "p", false, "Enable port forwarding")
	connectCmd.Flags().IntVarP(&remotePort, "remote-port", "r", 0, "Remote port (default 80)")
	connectCmd.Flags().IntVarP(&localPort, "local-port", "l", 0, "Local port (defaults to remote port)")
	connectCmd.Flags().StringVarP(&remoteHost, "remote-host", "H", "", "Remote host (for RDS/external host forwarding)")
	connectCmd.Flags().StringVarP(&configFile, "config", "f", "", "JSON config file for connection")

	rootCmd.AddCommand(connectCmd)
}
