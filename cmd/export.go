package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/awsini"
	"awsm/internal/tui"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var includeSecrets bool

var exportCmd = &cobra.Command{
	Use:   "export [output-file]",
	Short: "Export all profiles and SSO sessions to a file",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default output file
		outputFile := fmt.Sprintf("awsm-export-%s.json", time.Now().Format("2006-01-02-150405"))
		if len(args) > 0 {
			outputFile = args[0]
		}

		tui.PrintInfo(fmt.Sprintf("Exporting AWS configuration to: %s", tui.FormatBold(outputFile)))

		// Get profiles
		profiles, err := aws.ListProfilesDetailed()
		if err != nil {
			return fmt.Errorf("failed to list profiles: %w", err)
		}

		// Get SSO sessions
		ssoSessions, err := aws.ListSSOSessions()
		if err != nil {
			return fmt.Errorf("failed to list SSO sessions: %w", err)
		}

		var configContent, credentialsContent string
		if includeSecrets {
			// Read config file content
			configPath, _ := aws.GetAWSConfigPath()
			if data, err := os.ReadFile(configPath); err == nil {
				configContent = string(data)
			}

			// Read credentials file content
			credentialsPath, _ := aws.GetAWSCredentialsPath()
			if data, err := os.ReadFile(credentialsPath); err == nil {
				credentialsContent = string(data)
			}
		} else {
			// Redact secrets from profiles
			for i := range profiles {
				profiles[i].AccessKey = ""
				profiles[i].SecretKey = ""
			}
		}

		exportData := ExportData{
			ExportedAt:      time.Now(),
			Version:         "1.0",
			Profiles:        profiles,
			SSOSessions:     ssoSessions,
			ConfigFile:      configContent,
			CredentialsFile: credentialsContent,
		}

		data, err := json.MarshalIndent(exportData, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to encode export data: %w", err)
		}
		if err := awsini.WritePrivateFile(outputFile, append(data, '\n')); err != nil {
			return fmt.Errorf("failed to write export data: %w", err)
		}

		tui.PrintSuccess(fmt.Sprintf("Export complete: %d profiles, %d SSO sessions", len(profiles), len(ssoSessions)))
		tui.PrintMuted(fmt.Sprintf("File saved: %s", outputFile))
		return nil
	},
}

func init() {
	exportCmd.Flags().BoolVar(&includeSecrets, "include-secrets", false, "Include credentials and raw config files in the export")
	rootCmd.AddCommand(exportCmd)
}
