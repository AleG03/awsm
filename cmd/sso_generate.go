package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"awsm/internal/aws"
	awsmConfig "awsm/internal/config"
	"awsm/internal/util"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate <sso-session-name>",
	Short: "Generates AWS config profiles for all accessible SSO accounts and roles",
	Long: `This powerful command logs into an SSO session, discovers all accounts and roles
you have access to, and generates the corresponding AWS profile configurations.

The generated profiles are saved to '~/.aws/config' using the region from the SSO session.
Existing profiles are automatically updated without prompting.`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeSSOSessions,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSSOGenerate(args[0])
	},
}

func runSSOGenerate(ssoSession string) error {
	// Get region from SSO session configuration
	awsRegion, err := getSSORegionForSession(ssoSession)
	if err != nil {
		return fmt.Errorf("failed to get region from SSO session '%s': %w", ssoSession, err)
	}
	if !aws.IsValidRegion(awsRegion) {
		return fmt.Errorf("invalid region in SSO session: %s", awsRegion)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot find home directory: %w", err)
	}
	outputFile := filepath.Join(home, ".aws", "config")

	// 1. Log in to get a fresh token cached by the AWS CLI
	util.InfoColor.Printf("Attempting SSO login for session: %s\n", util.BoldColor.Sprint(ssoSession))
	awsCmd := exec.Command("aws", "sso", "login", "--sso-session", ssoSession)
	awsCmd.Stdin = os.Stdin
	awsCmd.Stdout = os.Stdout
	awsCmd.Stderr = os.Stderr
	if err := awsCmd.Run(); err != nil {
		return fmt.Errorf("aws sso login failed: %w", err)
	}
	util.SuccessColor.Println("\n✔ SSO login successful.")

	// 2. Find the cached access token from the filesystem
	util.InfoColor.Println("Finding cached SSO access token...")

	// Give the AWS CLI a moment to write the token cache
	time.Sleep(2 * time.Second)

	accessToken, err := findLatestSsoToken(filepath.Join(home, ".aws", "sso", "cache"))
	if err != nil {
		return fmt.Errorf("could not find cached SSO token: %w", err)
	}
	util.SuccessColor.Println("✔ Found access token.")

	// 3. Create SSO client with the region from session configuration

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(awsRegion))
	if err != nil {
		return fmt.Errorf("could not create basic AWS config: %w", err)
	}
	ssoClient := sso.NewFromConfig(cfg)

	// 4. List Accounts using the access token with retry logic
	util.InfoColor.Println("Fetching all accessible accounts...")
	var accounts []*sso.ListAccountsOutput

	// Add a small delay to ensure token is properly cached
	time.Sleep(1 * time.Second)

	accountsPaginator := sso.NewListAccountsPaginator(ssoClient, &sso.ListAccountsInput{
		AccessToken: &accessToken,
	})
	for accountsPaginator.HasMorePages() {
		page, err := accountsPaginator.NextPage(context.TODO())
		if err != nil {
			if strings.Contains(err.Error(), "UnauthorizedException") || strings.Contains(err.Error(), "401") {
				return fmt.Errorf("failed to list accounts: Session token not found or invalid.\n\nThis usually happens when:\n1. The SSO session has expired\n2. The cached token is stale\n3. There's a region mismatch\n\nTry running the command again, or clear your SSO cache with: rm -rf ~/.aws/sso/cache/*")
			}
			return fmt.Errorf("failed to list accounts: %w", err)
		}
		accounts = append(accounts, page)
	}

	totalAccounts := 0
	for _, page := range accounts {
		totalAccounts += len(page.AccountList)
	}
	util.SuccessColor.Printf("✔ Found %d accounts.\n", totalAccounts)

	// Read existing config
	existingConfig, err := awsmConfig.ReadConfigFile(outputFile)
	if err != nil {
		// If file doesn't exist, we'll start with empty config
		existingConfig = ""
	}

	// Parse existing profile names and their content
	existingProfiles, existingProfileContent := awsmConfig.ParseExistingProfiles(existingConfig)

	// Build new profiles
	var newProfilesBuilder strings.Builder
	cleaner := regexp.MustCompile(`[^a-zA-Z0-9-]`)
	profileCount := 0

	util.InfoColor.Println("Generating profiles...")
	for _, page := range accounts {
		for _, acc := range page.AccountList {
			util.InfoColor.Fprintf(os.Stderr, "  -> Processing account: %s (%s)\n", *acc.AccountName, *acc.AccountId)

			rolesPaginator := sso.NewListAccountRolesPaginator(ssoClient, &sso.ListAccountRolesInput{
				AccessToken: &accessToken,
				AccountId:   acc.AccountId,
			})
			for rolesPaginator.HasMorePages() {
				rolesPage, err := rolesPaginator.NextPage(context.TODO())
				if err != nil {
					util.ErrorColor.Fprintf(os.Stderr, "    Could not list roles for account %s: %v\n", *acc.AccountId, err)
					continue
				}
				for _, role := range rolesPage.RoleList {
					// Sanitize names for the profile
					cleanAccountName := strings.ToLower(*acc.AccountName)
					cleanAccountName = cleaner.ReplaceAllString(cleanAccountName, "-")
					cleanRoleName := strings.ToLower(*role.RoleName)
					cleanRoleName = cleaner.ReplaceAllString(cleanRoleName, "-")

					profileName := fmt.Sprintf("%s-%s", cleanAccountName, cleanRoleName)

					// Generate the new profile content
					newProfileContent := fmt.Sprintf("[profile %s]\nsso_session = %s\nsso_account_id = %s\nsso_role_name = %s\nregion = %s\n\n",
						profileName, ssoSession, *acc.AccountId, *role.RoleName, awsRegion)

					if existingProfiles[profileName] {
						// Check if the profile content is different
						if existingContent, exists := existingProfileContent[profileName]; exists {
							// Extract just the configuration lines for comparison
							existingLines := awsmConfig.ExtractProfileConfig(existingContent)
							newLines := awsmConfig.ExtractProfileConfig(newProfileContent)

							if existingLines == newLines {
								// Profile is identical, skip it
								util.InfoColor.Fprintf(os.Stderr, "    Profile '%s' is up to date, skipping\n", profileName)
								continue
							} else {
								// Profile content is different, update it
								util.InfoColor.Fprintf(os.Stderr, "    Updating profile '%s' with new configuration\n", profileName)
								// Remove the old profile from existing config
								existingConfig = awsmConfig.RemoveProfileFromConfig(existingConfig, profileName)
							}
						} else {
							// Profile exists but we couldn't find its content, update it anyway
							util.InfoColor.Fprintf(os.Stderr, "    Updating profile '%s'\n", profileName)
							existingConfig = awsmConfig.RemoveProfileFromConfig(existingConfig, profileName)
						}
					}

					newProfilesBuilder.WriteString(newProfileContent)
					profileCount++
				}
			}
		}
	}

	// Write the updated config
	if newProfilesBuilder.Len() > 0 {
		// Combine existing config (with removed profiles if updating) and new profiles
		finalConfig := existingConfig
		newContent := newProfilesBuilder.String()
		if len(finalConfig) > 0 {
			if !strings.HasSuffix(finalConfig, "\n") {
				finalConfig += "\n"
			}
			finalConfig += "\n" + newContent
		} else {
			finalConfig = newContent
		}

		// Write the complete config file
		if err := awsmConfig.WriteConfigFile(outputFile, finalConfig); err != nil {
			return fmt.Errorf("failed to write %s: %w", outputFile, err)
		}

		util.SuccessColor.Printf("\n✔ Done! %d profiles updated/added to %s\n", profileCount, util.BoldColor.Sprint(outputFile))
	} else {
		util.InfoColor.Println("All profiles are up to date.")
	}

	util.InfoColor.Println("You can now use the new profiles from your ~/.aws/config.")
	return nil
}

func getSSORegionForSession(ssoSession string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home directory: %w", err)
	}

	configPath := filepath.Join(home, ".aws", "config")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("could not read AWS config file: %w", err)
	}

	configContent := string(data)

	// Look for the SSO session section
	sessionPattern := fmt.Sprintf(`\[sso-session %s\]`, regexp.QuoteMeta(ssoSession))
	sessionRegex := regexp.MustCompile(sessionPattern)
	sessionMatch := sessionRegex.FindStringIndex(configContent)
	if sessionMatch == nil {
		return "", fmt.Errorf("SSO session '%s' not found in config", ssoSession)
	}

	// Find the region within this session section
	sessionStart := sessionMatch[1]
	nextSectionRegex := regexp.MustCompile(`\n\[`)
	nextSectionMatch := nextSectionRegex.FindStringIndex(configContent[sessionStart:])

	var sessionEnd int
	if nextSectionMatch != nil {
		sessionEnd = sessionStart + nextSectionMatch[0]
	} else {
		sessionEnd = len(configContent)
	}

	sessionSection := configContent[sessionStart:sessionEnd]
	regionRegex := regexp.MustCompile(`(?m)^sso_region\s*=\s*(.+)$`)
	regionMatch := regionRegex.FindStringSubmatch(sessionSection)
	if regionMatch != nil {
		return strings.TrimSpace(regionMatch[1]), nil
	}

	return "", fmt.Errorf("sso_region not found in session '%s'", ssoSession)
}

func findLatestSsoToken(cacheDir string) (string, error) {
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return "", fmt.Errorf("could not read SSO cache directory at %s: %w", cacheDir, err)
	}

	var latestFile os.FileInfo
	var latestTime time.Time
	var validToken string

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		info, err := file.Info()
		if err != nil {
			continue
		}

		// Read and parse each file to find valid access tokens
		fullPath := filepath.Join(cacheDir, file.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		// Try different token formats
		var tokenData map[string]interface{}
		if err := json.Unmarshal(data, &tokenData); err != nil {
			continue
		}

		// Look for accessToken in various formats
		var accessToken string
		if token, ok := tokenData["accessToken"].(string); ok && token != "" {
			accessToken = token
		} else if token, ok := tokenData["access_token"].(string); ok && token != "" {
			accessToken = token
		}

		// Check if token has expiration and if it's still valid
		if accessToken != "" {
			isValid := true
			if expiresAt, ok := tokenData["expiresAt"].(string); ok {
				if expTime, err := time.Parse(time.RFC3339, expiresAt); err == nil {
					if time.Now().After(expTime) {
						isValid = false
					}
				}
			}

			if isValid && info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestFile = info
				validToken = accessToken
			}
		}
	}

	if latestFile == nil || validToken == "" {
		return "", fmt.Errorf("no valid SSO token cache file found in %s.\n\nThis could mean:\n1. No SSO login has been performed\n2. All cached tokens have expired\n3. The cache directory is empty\n\nTry running 'aws sso login --sso-session <session-name>' first", cacheDir)
	}

	return validToken, nil
}

func init() {
	ssoCmd.AddCommand(generateCmd)
}
