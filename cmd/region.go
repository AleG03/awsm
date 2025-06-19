package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/tui"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
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
		fmt.Fprintln(cmd.ErrOrStderr(), tui.InfoStyle.Render("🌍 Fetching available AWS regions..."))

		cfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			return err
		}

		client := ec2.NewFromConfig(cfg)
		output, err := client.DescribeRegions(context.TODO(), &ec2.DescribeRegionsInput{})
		if err != nil {
			return err
		}

		if len(output.Regions) == 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), tui.WarningStyle.Render("⚠ No regions found."))
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), tui.HeaderStyle.Render("\n🌍 Available AWS Regions:"))
		for _, region := range output.Regions {
			fmt.Printf("  %s %s\n", tui.InfoStyle.Render("•"), *region.RegionName)
		}
		return nil
	},
}

var regionSetCmd = &cobra.Command{
	Use:   "set <region>",
	Short: "Set the region for the default profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		region := args[0]

		err := aws.SetRegion(region)
		if err != nil {
			return fmt.Errorf("failed to set region: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStderr(), tui.SuccessStyle.Render("✓ Region set to '"+region+"' in default profile."))
		return nil
	},
}

func init() {
	regionCmd.AddCommand(regionListCmd)
	regionCmd.AddCommand(regionSetCmd)
	rootCmd.AddCommand(regionCmd)
}
