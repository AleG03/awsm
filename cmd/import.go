package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/util"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type ExportData struct {
	ExportedAt      time.Time            `json:"exported_at"`
	Version         string               `json:"version"`
	Profiles        []aws.ProfileInfo    `json:"profiles"`
	SSOSessions     []aws.SSOSessionInfo `json:"sso_sessions"`
	ConfigFile      string               `json:"config_file,omitempty"`
	CredentialsFile string               `json:"credentials_file,omitempty"`
}

var (
	importForce bool
)

var importCmd = &cobra.Command{
	Use:   "import <export-file>",
	Short: "Import profiles and SSO sessions from an export file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		importFile := args[0]

		util.InfoColor.Printf("Importing AWS configuration from: %s\n", util.BoldColor.Sprint(importFile))

		// Read import file
		file, err := os.Open(importFile)
		if err != nil {
			return fmt.Errorf("failed to open import file: %w", err)
		}
		defer file.Close()

		var exportData ExportData
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&exportData); err != nil {
			return fmt.Errorf("failed to parse import file: %w", err)
		}

		util.InfoColor.Printf("Import file contains: %d profiles, %d SSO sessions\n",
			len(exportData.Profiles), len(exportData.SSOSessions))

		if !importForce {
			confirm, err := util.PromptForInput("Continue with import? This may overwrite existing configurations (y/N): ")
			if err != nil {
				return err
			}
			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				util.InfoColor.Println("Import cancelled")
				return nil
			}
		}

		// Import SSO sessions first
		importedSessions := 0
		for _, session := range exportData.SSOSessions {
			if err := aws.ImportSSOSession(session); err != nil {
				util.ErrorColor.Printf("Failed to import SSO session '%s': %v\n", session.Name, err)
			} else {
				util.SuccessColor.Printf("✔ Imported SSO session '%s'\n", session.Name)
				importedSessions++
			}
		}

		// Import profiles
		importedProfiles := 0
		for _, profile := range exportData.Profiles {
			if err := aws.ImportProfile(profile); err != nil {
				util.ErrorColor.Printf("Failed to import profile '%s': %v\n", profile.Name, err)
			} else {
				util.SuccessColor.Printf("✔ Imported profile '%s'\n", profile.Name)
				importedProfiles++
			}
		}

		util.SuccessColor.Printf("✔ Import complete: %d profiles, %d SSO sessions imported\n",
			importedProfiles, importedSessions)
		return nil
	},
}

func init() {
	importCmd.Flags().BoolVarP(&importForce, "force", "f", false, "Force import without confirmation")
	rootCmd.AddCommand(importCmd)
}
