package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/tui"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var profileAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new AWS profile",
}

var profileAddIAMUserCmd = &cobra.Command{
	Use:   "iam-user <profile-name>",
	Short: "Add an IAM user profile with static credentials",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		// Check if profile already exists
		if exists, err := aws.ProfileExists(profileName); err != nil {
			return fmt.Errorf("failed to check if profile exists: %w", err)
		} else if exists {
			resolvedName, skip := resolveProfileAddConflict(profileName, "iam-user")
			if skip {
				tui.PrintMuted("Profile creation cancelled.")
				return nil
			}
			profileName = resolvedName
		}

		tui.PrintInfo(fmt.Sprintf("Adding IAM user profile: %s", tui.FormatBold(profileName)))

		accessKey, err := tui.PromptInput("AWS Access Key ID", tui.WithRequired(), tui.WithPlaceholder("AKIA..."))
		if err != nil {
			return err
		}

		secretKey, err := tui.PromptInput("AWS Secret Access Key", tui.WithRequired(), tui.WithEchoPassword(), tui.WithPlaceholder("your-secret-key"))
		if err != nil {
			return err
		}

		region, err := tui.SelectRegion()
		if err != nil {
			return err
		}

		if err := aws.AddIAMUserProfile(profileName, accessKey, secretKey, region); err != nil {
			return fmt.Errorf("failed to add IAM user profile: %w", err)
		}

		tui.PrintSuccess(fmt.Sprintf("IAM user profile '%s' added successfully", profileName))
		offerToRestrictConfig()
		return nil
	},
}

var profileAddIAMRoleCmd = &cobra.Command{
	Use:   "iam-role <profile-name>",
	Short: "Add an IAM role profile with role assumption",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		// Check if profile already exists
		if exists, err := aws.ProfileExists(profileName); err != nil {
			return fmt.Errorf("failed to check if profile exists: %w", err)
		} else if exists {
			resolvedName, skip := resolveProfileAddConflict(profileName, "iam-role")
			if skip {
				tui.PrintMuted("Profile creation cancelled.")
				return nil
			}
			profileName = resolvedName
		}

		tui.PrintInfo(fmt.Sprintf("Adding IAM role profile: %s", tui.FormatBold(profileName)))

		roleArn, err := tui.PromptInput("Role ARN",
			tui.WithRequired(),
			tui.WithPlaceholder("arn:aws:iam::123456789012:role/MyRole"))
		if err != nil {
			return err
		}

		sourceProfile, err := tui.PromptInput("Source profile",
			tui.WithPlaceholder("optional - leave empty to skip"))
		if err != nil {
			return err
		}

		mfaSerial, err := tui.PromptInput("MFA Serial",
			tui.WithPlaceholder("optional - arn:aws:iam::...:mfa/user"))
		if err != nil {
			return err
		}

		externalID, err := tui.PromptInput("External ID",
			tui.WithPlaceholder("optional - required by some cross-account roles"))
		if err != nil {
			return err
		}

		region, err := tui.SelectRegion()
		if err != nil {
			return err
		}

		if err := aws.AddIAMRoleProfile(profileName, aws.IAMRoleProfile{
			RoleARN:       roleArn,
			SourceProfile: strings.TrimSpace(sourceProfile),
			MFASerial:     strings.TrimSpace(mfaSerial),
			ExternalID:    strings.TrimSpace(externalID),
			Region:        region,
		}); err != nil {
			return fmt.Errorf("failed to add IAM role profile: %w", err)
		}

		tui.PrintSuccess(fmt.Sprintf("IAM role profile '%s' added successfully", profileName))
		return nil
	},
}

func resolveProfileAddConflict(profileName, profileType string) (string, bool) {
	tui.PrintWarning(fmt.Sprintf("Profile '%s' already exists.", profileName))

	choice, err := tui.SelectChoice("Choose resolution:", []tui.Choice{
		{Label: "Skip this profile", Value: "skip"},
		{Label: fmt.Sprintf("Rename to '%s-%s'", profileName, profileType), Value: "rename"},
		{Label: "Enter custom name", Value: "custom"},
		{Label: "Overwrite existing profile", Value: "overwrite"},
	})
	if err != nil {
		return "", true
	}

	switch choice.Value {
	case "skip":
		return "", true
	case "rename":
		return fmt.Sprintf("%s-%s", profileName, profileType), false
	case "custom":
		customName, err := tui.PromptInput("New profile name", tui.WithRequired())
		if err != nil {
			return "", true
		}
		return customName, false
	case "overwrite":
		return profileName, false
	default:
		return "", true
	}
}

func init() {
	profileAddCmd.AddCommand(profileAddIAMUserCmd)
	profileAddCmd.AddCommand(profileAddIAMRoleCmd)
	profileCmd.AddCommand(profileAddCmd)
}
