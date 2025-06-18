package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/util"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	listDetailed bool
	filterType   string
	filterRegion string
	nameFilter   string
	sortBy       string
	showHelp     bool
	outputJSON   bool
)

// JSONProfileInfo represents the profile information in a scripting-friendly format
type JSONProfileInfo struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Region        string `json:"region"`
	AccountID     string `json:"account_id,omitempty"`
	RoleARN       string `json:"role_arn,omitempty"`
	SourceProfile string `json:"source_profile,omitempty"`
	SSOStartURL   string `json:"sso_start_url,omitempty"`
	SSORegion     string `json:"sso_region,omitempty"`
	SSOAccountID  string `json:"sso_account_id,omitempty"`
	SSORoleName   string `json:"sso_role_name,omitempty"`
	SSOSession    string `json:"sso_session,omitempty"`
	MFASerial     string `json:"mfa_serial,omitempty"`
	IsActive      bool   `json:"is_active"`
}

// Profile type descriptions
var profileTypeDescriptions = map[aws.ProfileType]string{
	aws.ProfileTypeSSO: "AWS IAM Identity Center (SSO) profile - Uses SSO for authentication",
	aws.ProfileTypeIAM: "IAM profile with role assumption - Requires MFA and assumes a role",
	aws.ProfileTypeKey: "Static credentials - Uses AWS access key and secret",
}

var profileCmd = &cobra.Command{
	Use:     "profile",
	Short:   "Manage AWS profiles",
	Aliases: []string{"p"},
}

var profileListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all available AWS profiles",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if showHelp {
			printProfileTypeHelp()
			return nil
		}

		profiles, err := aws.ListProfilesDetailed()
		if err != nil {
			return err
		}

		if len(profiles) == 0 {
			if !outputJSON {
				util.WarnColor.Println("No profiles found.")
			} else {
				fmt.Println("[]") // Empty JSON array
			}
			return nil
		}

		// Apply filters
		var filtered []aws.ProfileInfo
		for _, p := range profiles {
			if filterType != "" && !strings.EqualFold(string(p.Type), filterType) {
				continue
			}
			if filterRegion != "" && !strings.EqualFold(p.Region, filterRegion) {
				continue
			}
			if nameFilter != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(nameFilter)) {
				continue
			}
			filtered = append(filtered, p)
		}

		if len(filtered) == 0 {
			if !outputJSON {
				util.WarnColor.Println("No profiles match the specified filters.")
			} else {
				fmt.Println("[]") // Empty JSON array
			}
			return nil
		}

		// Sort profiles based on sortBy flag
		switch strings.ToLower(sortBy) {
		case "type":
			util.SortBy(filtered, func(p1, p2 aws.ProfileInfo) bool {
				return string(p1.Type) < string(p2.Type)
			})
		case "region":
			util.SortBy(filtered, func(p1, p2 aws.ProfileInfo) bool {
				return p1.Region < p2.Region
			})
		default: // default sort by name
			util.SortBy(filtered, func(p1, p2 aws.ProfileInfo) bool {
				return p1.Name < p2.Name
			})
		}

		if outputJSON {
			return outputProfilesJSON(filtered)
		}

		// Print profiles
		if listDetailed {
			printDetailedProfiles(filtered)
		} else {
			printSimpleProfiles(filtered)
		}

		return nil
	},
}

func outputProfilesJSON(profiles []aws.ProfileInfo) error {
	var jsonProfiles []JSONProfileInfo

	for _, p := range profiles {
		// Extract account ID from role ARN if available
		accountID := p.SSOAccountID
		if accountID == "" && p.RoleARN != "" {
			parts := strings.Split(p.RoleARN, ":")
			if len(parts) >= 5 {
				accountID = parts[4]
			}
		}

		jsonProfile := JSONProfileInfo{
			Name:          p.Name,
			Type:          string(p.Type),
			Region:        p.Region,
			AccountID:     accountID,
			RoleARN:       p.RoleARN,
			SourceProfile: p.SourceProfile,
			SSOStartURL:   p.SSOStartURL,
			SSORegion:     p.SSORegion,
			SSOAccountID:  p.SSOAccountID,
			SSORoleName:   p.SSORoleName,
			SSOSession:    p.SSOSession,
			MFASerial:     p.MFASerial,
			IsActive:      p.IsActive,
		}
		jsonProfiles = append(jsonProfiles, jsonProfile)
	}

	// Create JSON encoder with indentation for readability
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonProfiles)
}

func printProfileTypeHelp() {
	util.InfoColor.Println("AWS Profile Types:")
	fmt.Println()
	for pType, desc := range profileTypeDescriptions {
		fmt.Printf("%s: %s\n", colorizeProfileType(pType), desc)
	}
	fmt.Println()
}

func colorizeProfileType(pType aws.ProfileType) string {
	switch pType {
	case aws.ProfileTypeSSO:
		return util.SuccessColor.Sprint(pType)
	case aws.ProfileTypeIAM:
		return util.InfoColor.Sprint(pType)
	default:
		return util.WarnColor.Sprint(pType)
	}
}

func printSimpleProfiles(profiles []aws.ProfileInfo) {
	fmt.Println()
	util.InfoColor.Println("AWS Profiles")
	util.InfoColor.Println("═══════════")
	fmt.Println()

	var ssoProfiles, iamProfiles, keyProfiles []aws.ProfileInfo
	for _, p := range profiles {
		switch p.Type {
		case aws.ProfileTypeSSO:
			ssoProfiles = append(ssoProfiles, p)
		case aws.ProfileTypeIAM:
			iamProfiles = append(iamProfiles, p)
		case aws.ProfileTypeKey:
			keyProfiles = append(keyProfiles, p)
		}
	}

	// Print SSO Profiles
	if len(ssoProfiles) > 0 {
		util.SuccessColor.Println("● SSO Profiles")
		for _, p := range ssoProfiles {
			fmt.Print("  ")
			if p.IsActive {
				util.SuccessColor.Print("▶ ")
			} else {
				fmt.Print("  ")
			}
			fmt.Printf("%s ", p.Name)
			if p.SSOAccountID != "" {
				util.InfoColor.Printf("(%s) ", p.SSOAccountID)
			}
			util.WarnColor.Printf("[%s]\n", p.Region)
		}
		fmt.Println()
	}

	// Print IAM Profiles
	if len(iamProfiles) > 0 {
		util.InfoColor.Println("● IAM Role Profiles")
		for _, p := range iamProfiles {
			fmt.Print("  ")
			if p.IsActive {
				util.SuccessColor.Print("▶ ")
			} else {
				fmt.Print("  ")
			}
			fmt.Printf("%s ", p.Name)
			if p.RoleARN != "" {
				parts := strings.Split(p.RoleARN, ":")
				if len(parts) >= 5 {
					util.InfoColor.Printf("(%s) ", parts[4])
				}
			}
			util.WarnColor.Printf("[%s]\n", p.Region)
		}
		fmt.Println()
	}

	// Print Key Profiles
	if len(keyProfiles) > 0 {
		util.WarnColor.Println("● Static Key Profiles")
		for _, p := range keyProfiles {
			fmt.Print("  ")
			if p.IsActive {
				util.SuccessColor.Print("▶ ")
			} else {
				fmt.Print("  ")
			}
			fmt.Printf("%s ", p.Name)
			util.WarnColor.Printf("[%s]\n", p.Region)
		}
		fmt.Println()
	}

	// Print legend
	fmt.Println("Legend:")
	util.SuccessColor.Print("▶ ")
	fmt.Println("Active profile")
	fmt.Print("● ")
	fmt.Println("Profile type indicator")
	util.InfoColor.Print("(123456789012) ")
	fmt.Println("AWS Account ID")
	util.WarnColor.Print("[us-east-1] ")
	fmt.Println("Region")
}

func printDetailedProfiles(profiles []aws.ProfileInfo) {
	fmt.Println()
	util.InfoColor.Println("AWS Profiles (Detailed)")
	util.InfoColor.Println("═════════════════════")
	fmt.Println()

	for i, p := range profiles {
		// Profile name and status
		if p.IsActive {
			util.SuccessColor.Print("▶ ")
		} else {
			fmt.Print("  ")
		}
		util.InfoColor.Printf("Profile: %s\n", p.Name)

		// Type-specific details with indent
		fmt.Print("    ")
		switch p.Type {
		case aws.ProfileTypeSSO:
			util.SuccessColor.Print("● SSO Profile\n")
			fmt.Printf("    ├── Account: %s\n", p.SSOAccountID)
			fmt.Printf("    ├── Region: %s\n", p.Region)
			if p.SSOSession != "" {
				fmt.Printf("    ├── Session: %s\n", p.SSOSession)
			}
			if p.SSORoleName != "" {
				fmt.Printf("    └── Role: %s\n", p.SSORoleName)
			}

		case aws.ProfileTypeIAM:
			util.InfoColor.Print("● IAM Profile\n")
			if p.RoleARN != "" {
				fmt.Printf("    ├── Role: %s\n", p.RoleARN)
			}
			fmt.Printf("    ├── Region: %s\n", p.Region)
			if p.MFASerial != "" {
				fmt.Printf("    ├── MFA: %s\n", p.MFASerial)
			}
			if p.SourceProfile != "" {
				fmt.Printf("    └── Source: %s\n", p.SourceProfile)
			}

		case aws.ProfileTypeKey:
			util.WarnColor.Print("● Static Key Profile\n")
			fmt.Printf("    └── Region: %s\n", p.Region)
		}

		// Add spacing between profiles
		if i < len(profiles)-1 {
			fmt.Println()
		}
	}
	fmt.Println()
}

func init() {
	profileListCmd.Flags().BoolVarP(&listDetailed, "detailed", "d", false, "Show detailed profile information")
	profileListCmd.Flags().StringVarP(&filterType, "type", "t", "", "Filter by profile type (SSO, IAM, Key)")
	profileListCmd.Flags().StringVarP(&filterRegion, "region", "r", "", "Filter by region")
	profileListCmd.Flags().StringVarP(&nameFilter, "name", "n", "", "Filter by profile name (case-insensitive)")
	profileListCmd.Flags().StringVarP(&sortBy, "sort", "s", "name", "Sort by field (name, type, region)")
	profileListCmd.Flags().BoolVarP(&showHelp, "help-types", "H", false, "Show help about profile types")
	profileListCmd.Flags().BoolVarP(&outputJSON, "json", "j", false, "Output profiles in JSON format")

	profileCmd.AddCommand(profileListCmd)
	rootCmd.AddCommand(profileCmd)
}
