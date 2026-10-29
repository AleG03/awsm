package aws

import (
	"fmt"
	"strings"
)

// AWSRegions contains all valid AWS regions
var AWSRegions = map[string]bool{
	"us-east-1":      true,
	"us-east-2":      true,
	"us-west-1":      true,
	"us-west-2":      true,
	"af-south-1":     true,
	"ap-east-1":      true,
	"ap-south-2":     true,
	"ap-southeast-3": true,
	"ap-southeast-5": true,
	"ap-southeast-4": true,
	"ap-south-1":     true,
	"ap-southeast-6": true,
	"ap-northeast-3": true,
	"ap-northeast-2": true,
	"ap-southeast-1": true,
	"ap-southeast-2": true,
	"ap-east-2":      true,
	"ap-southeast-7": true,
	"ap-northeast-1": true,
	"ca-central-1":   true,
	"ca-west-1":      true,
	"eu-central-1":   true,
	"eu-west-1":      true,
	"eu-west-2":      true,
	"eu-south-1":     true,
	"eu-west-3":      true,
	"eu-south-2":     true,
	"eu-north-1":     true,
	"eu-central-2":   true,
	"il-central-1":   true,
	"mx-central-1":   true,
	"me-south-1":     true,
	"me-central-1":   true,
	"sa-east-1":      true,
	"eusc-de-east-1": true,
}

// euscRegionPrefix marks the regions of the AWS European Sovereign Cloud, the
// aws-eusc partition.
//
// It is a separate cloud rather than another region: its own domains, its own
// IAM Identity Center, and credentials that the commercial partition does not
// recognise. Anything awsm addresses by URL instead of through the SDK has to
// follow the partition, or it sends sovereign credentials to a commercial
// endpoint that will only ever answer 400.
const euscRegionPrefix = "eusc-"

// ConsoleFederationURL is the endpoint that exchanges credentials for a console
// sign-in token in the partition the region belongs to.
//
// The sovereign one is regional on purpose: signin.amazonaws-eusc.eu has no DNS
// record, so a global host modelled on signin.aws.amazon.com cannot work.
func ConsoleFederationURL(region string) string {
	if strings.HasPrefix(region, euscRegionPrefix) {
		return fmt.Sprintf("https://%s.signin.amazonaws-eusc.eu/federation", region)
	}
	return "https://signin.aws.amazon.com/federation"
}

// ConsoleHomeURL is the console landing page for a region.
func ConsoleHomeURL(region string) string {
	if strings.HasPrefix(region, euscRegionPrefix) {
		return fmt.Sprintf("https://%s.console.amazonaws-eusc.eu/console/home?region=%s", region, region)
	}
	return fmt.Sprintf("https://%s.console.aws.amazon.com/console/home?region=%s", region, region)
}

// IsValidRegion checks if the given region is valid
func IsValidRegion(region string) bool {
	return AWSRegions[region]
}

// GetAllRegions returns a slice of all valid AWS regions
func GetAllRegions() []string {
	regions := make([]string, 0, len(AWSRegions))
	for region := range AWSRegions {
		regions = append(regions, region)
	}
	return regions
}
