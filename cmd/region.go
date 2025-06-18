package cmd

import (
	"awsm/internal/util"
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
		util.InfoColor.Println("Fetching available AWS regions...")

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
			util.WarnColor.Println("No regions found.")
			return nil
		}

		util.InfoColor.Println("\nAvailable AWS Regions:")
		for _, region := range output.Regions {
			fmt.Printf("└─ %s\n", *region.RegionName)
		}
		return nil
	},
}

func init() {
	regionCmd.AddCommand(regionListCmd)
	rootCmd.AddCommand(regionCmd)
}
