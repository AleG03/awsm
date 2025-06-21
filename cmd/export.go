package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/util"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

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

		util.InfoColor.Printf("Exporting AWS configuration to: %s\n", util.BoldColor.Sprint(outputFile))

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

		// Read config file content
		configPath, _ := aws.GetAWSConfigPath()
		var configContent string
		if data, err := os.ReadFile(configPath); err == nil {
			configContent = string(data)
		}

		// Read credentials file content
		credentialsPath, _ := aws.GetAWSCredentialsPath()
		var credentialsContent string
		if data, err := os.ReadFile(credentialsPath); err == nil {
			credentialsContent = string(data)
		}

		exportData := ExportData{
			ExportedAt:      time.Now(),
			Version:         "1.0",
			Profiles:        profiles,
			SSOSessions:     ssoSessions,
			ConfigFile:      configContent,
			CredentialsFile: credentialsContent,
		}

		// Create output file
		file, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(exportData); err != nil {
			return fmt.Errorf("failed to write export data: %w", err)
		}

		util.SuccessColor.Printf("✔ Export complete: %d profiles, %d SSO sessions\n", len(profiles), len(ssoSessions))
		util.InfoColor.Printf("File saved: %s\n", outputFile)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
}
