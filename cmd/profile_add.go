package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/util"
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
				util.InfoColor.Println("Profile creation cancelled.")
				return nil
			}
			profileName = resolvedName
		}

		util.InfoColor.Printf("Adding IAM user profile: %s\n", util.BoldColor.Sprint(profileName))

		accessKey, err := util.PromptForInput("AWS Access Key ID: ")
		if err != nil {
			return err
		}
		if strings.TrimSpace(accessKey) == "" {
			return fmt.Errorf("access key ID is required")
		}

		secretKey, err := util.PromptForInput("AWS Secret Access Key: ")
		if err != nil {
			return err
		}
		if strings.TrimSpace(secretKey) == "" {
			return fmt.Errorf("secret access key is required")
		}

		region, err := util.PromptForInput("Default region (e.g., us-east-1): ")
		if err != nil {
			return err
		}
		if strings.TrimSpace(region) == "" {
			return fmt.Errorf("region is required")
		}
		if !aws.IsValidRegion(region) {
			return fmt.Errorf("invalid region: %s", region)
		}

		if err := aws.AddIAMUserProfile(profileName, accessKey, secretKey, region); err != nil {
			return fmt.Errorf("failed to add IAM user profile: %w", err)
		}

		util.SuccessColor.Printf("✔ IAM user profile '%s' added successfully\n", profileName)
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
				util.InfoColor.Println("Profile creation cancelled.")
				return nil
			}
			profileName = resolvedName
		}

		util.InfoColor.Printf("Adding IAM role profile: %s\n", util.BoldColor.Sprint(profileName))

		roleArn, err := util.PromptForInput("Role ARN: ")
		if err != nil {
			return err
		}

		sourceProfile, err := util.PromptForInput("Source profile (optional): ")
		if err != nil {
			return err
		}

		mfaSerial, err := util.PromptForInput("MFA Serial (optional): ")
		if err != nil {
			return err
		}

		region, err := util.PromptForInput("Default region (e.g., us-east-1): ")
		if err != nil {
			return err
		}
		if !aws.IsValidRegion(region) {
			return fmt.Errorf("invalid region: %s", region)
		}

		if err := aws.AddIAMRoleProfile(profileName, roleArn, strings.TrimSpace(sourceProfile), strings.TrimSpace(mfaSerial), region); err != nil {
			return fmt.Errorf("failed to add IAM role profile: %w", err)
		}

		util.SuccessColor.Printf("✔ IAM role profile '%s' added successfully\n", profileName)
		return nil
	},
}

func resolveProfileAddConflict(profileName, profileType string) (string, bool) {
	util.WarnColor.Printf("\n⚠ Profile '%s' already exists.\n", profileName)
	fmt.Println("Choose resolution:")
	fmt.Println("1. Skip this profile")
	fmt.Printf("2. Rename to '%s-%s'\n", profileName, profileType)
	fmt.Println("3. Enter custom name")
	fmt.Println("4. Overwrite existing profile")
	fmt.Print("\nEnter choice (1-4): ")

	choice, err := util.PromptForInput("")
	if err != nil {
		return "", true
	}

	switch strings.TrimSpace(choice) {
	case "1":
		return "", true
	case "2":
		return fmt.Sprintf("%s-%s", profileName, profileType), false
	case "3":
		customName, err := util.PromptForInput("Enter new profile name: ")
		if err != nil || strings.TrimSpace(customName) == "" {
			return "", true
		}
		return strings.TrimSpace(customName), false
	case "4":
		return profileName, false
	default:
		util.WarnColor.Println("Invalid choice. Skipping profile.")
		return "", true
	}
}

func init() {
	profileAddCmd.AddCommand(profileAddIAMUserCmd)
	profileAddCmd.AddCommand(profileAddIAMRoleCmd)
	profileCmd.AddCommand(profileAddCmd)
}
