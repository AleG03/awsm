package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/tui"
	"fmt"

	"github.com/spf13/cobra"
)

var regionCmd = &cobra.Command{
	Use:     "region",
	Short:   "Manage AWS regions",
	Aliases: []string{"r"},
}

var regionListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all available AWS regions",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		regions := aws.GetAllRegions()

		if len(regions) == 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), tui.WarningStyle.Render("⚠ No regions found."))
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), tui.HeaderStyle.Render("\n🌍 Available AWS Regions:"))
		for _, region := range regions {
			fmt.Printf("  %s %s\n", tui.InfoStyle.Render("•"), region)
		}
		return nil
	},
}

var regionSetCmd = &cobra.Command{
	Use:               "set <region>",
	Short:             "Set the region for the default profile",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeRegions,
	RunE: func(cmd *cobra.Command, args []string) error {
		region := args[0]

		if !aws.IsValidRegion(region) {
			return fmt.Errorf("invalid region: %s", region)
		}

		err := aws.SetRegion(region)
		if err != nil {
			return fmt.Errorf("failed to set region: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStderr(), tui.SuccessStyle.Render("✓ Region set to '"+region+"' in default profile."))
		return nil
	},
}

func completeRegions(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	regions := aws.GetAllRegions()
	return regions, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	regionCmd.AddCommand(regionListCmd)
	regionCmd.AddCommand(regionSetCmd)
	rootCmd.AddCommand(regionCmd)
}
