package aws

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
