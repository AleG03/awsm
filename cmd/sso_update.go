package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"awsm/internal/aws"
	awsmConfig "awsm/internal/config"
	"awsm/internal/tui"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/spf13/cobra"
)

var ssoUpdateCmd = &cobra.Command{
	Use:   "update <sso-session-name>",
	Short: "Syncs AWS config profiles for all accessible SSO accounts and roles",
	Long: `Logs into an SSO session, discovers all accounts and roles you have access to,
and syncs the corresponding AWS profile configurations.

Profiles are saved to '~/.aws/config' grouped by session and account.
Stale profiles (roles no longer accessible) are automatically removed.`,
	Aliases:           []string{"generate"},
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeSSOSessions,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSSOUpdate(args[0])
	},
}

func runSSOUpdate(ssoSession string) error {
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
	if err := aws.PerformSSOLogin(ssoSession); err != nil {
		return err
	}

	// 2. Find the cached access token from the filesystem
	tui.PrintStep("Finding cached SSO access token...")

	accessToken, err := findLatestSsoToken(filepath.Join(home, ".aws", "sso", "cache"))
	if err != nil {
		return fmt.Errorf("could not find cached SSO token: %w", err)
	}
	tui.PrintSuccess("Found access token.")

	// 3. Create SSO client with the region from session configuration

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(awsRegion))
	if err != nil {
		return fmt.Errorf("could not create basic AWS config: %w", err)
	}
	ssoClient := sso.NewFromConfig(cfg)

	// 4. List Accounts using the access token
	tui.PrintStep("Fetching all accessible accounts...")
	var accounts []*sso.ListAccountsOutput

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
	tui.PrintSuccess(fmt.Sprintf("Found %d accounts.", totalAccounts))

	// Read existing config
	existingConfig, err := awsmConfig.ReadConfigFile(outputFile)
	if err != nil {
		existingConfig = ""
	}

	// Remove all existing profiles for this SSO session (clean slate approach)
	tui.PrintStep(fmt.Sprintf("Removing old profiles for session '%s'...", ssoSession))
	existingConfig, removedProfiles := awsmConfig.RemoveAllProfilesForSession(existingConfig, ssoSession)
	if len(removedProfiles) > 0 {
		tui.PrintMuted(fmt.Sprintf("Removed %d old profiles.", len(removedProfiles)))
	}

	// Build new profiles from discovered accounts/roles
	var newProfilesBuilder strings.Builder
	cleaner := regexp.MustCompile(`[^a-zA-Z0-9-]`)
	profileCount := 0

	tui.PrintStep("Generating profiles...")
	for _, page := range accounts {
		for _, acc := range page.AccountList {
			tui.PrintStep(fmt.Sprintf("Processing account: %s (%s)", *acc.AccountName, *acc.AccountId))

			rolesPaginator := sso.NewListAccountRolesPaginator(ssoClient, &sso.ListAccountRolesInput{
				AccessToken: &accessToken,
				AccountId:   acc.AccountId,
			})
			for rolesPaginator.HasMorePages() {
				rolesPage, err := rolesPaginator.NextPage(context.TODO())
				if err != nil {
					tui.PrintError(fmt.Sprintf("Could not list roles for account %s: %v", *acc.AccountId, err))
					continue
				}
				for _, role := range rolesPage.RoleList {
					cleanSessionName := strings.ToLower(ssoSession)
					cleanSessionName = cleaner.ReplaceAllString(cleanSessionName, "-")
					cleanAccountName := strings.ToLower(*acc.AccountName)
					cleanAccountName = cleaner.ReplaceAllString(cleanAccountName, "-")
					cleanRoleName := strings.ToLower(*role.RoleName)
					cleanRoleName = cleaner.ReplaceAllString(cleanRoleName, "-")

					profileName := fmt.Sprintf("%s-%s-%s", cleanSessionName, cleanAccountName, cleanRoleName)

					newProfileContent := fmt.Sprintf("[profile %s]\nsso_session = %s\nsso_account_id = %s\nsso_role_name = %s\nregion = %s\n\n",
						profileName, ssoSession, *acc.AccountId, *role.RoleName, awsRegion)

					newProfilesBuilder.WriteString(newProfileContent)
					profileCount++
				}
			}
		}
	}

	// Write the updated config
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

	if err := awsmConfig.WriteConfigFile(outputFile, finalConfig); err != nil {
		return fmt.Errorf("failed to write %s: %w", outputFile, err)
	}

	// Reorganize the config file for readability
	tui.PrintStep("Organizing config file...")
	if err := awsmConfig.OrganizeConfigFile(outputFile); err != nil {
		tui.PrintWarning(fmt.Sprintf("Could not organize config file: %v", err))
	}

	tui.PrintSuccess(fmt.Sprintf("Done! %d profiles synced to %s", profileCount, tui.FormatBold(outputFile)))
	if len(removedProfiles) > profileCount {
		stale := len(removedProfiles) - profileCount
		tui.PrintInfo(fmt.Sprintf("Cleaned up %d stale profiles no longer accessible.", stale))
	}

	tui.PrintMuted("You can now use the new profiles from your ~/.aws/config.")
	return nil
}

func getSSORegionForSession(ssoSession string) (string, error) {
	sessions, err := aws.ListSSOSessions()
	if err != nil {
		return "", fmt.Errorf("could not list SSO sessions: %w", err)
	}
	for _, s := range sessions {
		if s.Name == ssoSession {
			if s.Region == "" {
				return "", fmt.Errorf("sso_region not found in session '%s'", ssoSession)
			}
			return s.Region, nil
		}
	}
	return "", fmt.Errorf("SSO session '%s' not found in config", ssoSession)
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
	ssoCmd.AddCommand(ssoUpdateCmd)
}
