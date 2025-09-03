package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"awsm/internal/aws"
	"awsm/internal/browser"
	"awsm/internal/util"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/spf13/cobra"
)

var (
	dontOpenBrowser bool
	useFirefox      bool
	chromeProfile   string
)

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Opens the AWS console in your browser",
	Long: `Generates a federated sign-in URL for the current AWS session and
automatically opens it in your default browser, a specified Chrome profile,
or a Firefox Multi-Account Container.

The command generates a console URL using your current AWS credentials.
Make sure your credentials are valid before running this command.

By default, opens in the default browser.
Use --chrome-profile to open in a specific Chrome profile.
Use --firefox-container to open in a Firefox container matching your AWS profile name.

Make sure to set a session first with 'awsm profile set <profile-name>'.`,
	Aliases: []string{"c", "open"},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get current profile
		currentProfile := os.Getenv("AWS_PROFILE")
		if currentProfile == "" {
			currentProfile = aws.GetCurrentProfileName()
		}
		if currentProfile == "" {
			return fmt.Errorf("no AWS profile set. Please run 'awsm profile set <profile-name>' first")
		}

		// Note: Credential validation will happen when AWS config is loaded

		// Load AWS config with the current profile
		awsCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(currentProfile))
		if err != nil {
			return fmt.Errorf("failed to load AWS config for profile '%s': %w\n\nPlease check your profile configuration with:\n  awsm profile list --detailed", currentProfile, err)
		}

		// Retrieve credentials
		creds, err := awsCfg.Credentials.Retrieve(context.TODO())
		if err != nil {
			// Check if this is a credential expiration error
			if strings.Contains(err.Error(), "expired") || strings.Contains(err.Error(), "InvalidGrantException") {
				return fmt.Errorf("credentials are expired: %w\n\nPlease try:\n  awsm sso login <session-name>\n  aws sso login --sso-session <session-name>", err)
			}
			return fmt.Errorf("failed to retrieve credentials for profile '%s': %w\n\nPlease check your profile configuration with:\n  awsm profile list --detailed", currentProfile, err)
		}

		if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
			return fmt.Errorf("invalid credentials found for profile '%s'. Please run:\n  awsm sso login <session-name>\n  awsm profile set <profile-name>", currentProfile)
		}
		sessionJSON, err := json.Marshal(map[string]string{"sessionId": creds.AccessKeyID, "sessionKey": creds.SecretAccessKey, "sessionToken": creds.SessionToken})
		if err != nil {
			return fmt.Errorf("failed to create session JSON: %w", err)
		}

		// Use POST to avoid URL length issues with long session tokens
		formData := url.Values{}
		formData.Set("Action", "getSigninToken")
		formData.Set("Session", string(sessionJSON))

		resp, err := http.PostForm("https://signin.aws.amazon.com/federation", formData)
		if err != nil {
			return fmt.Errorf("failed to get sign-in token: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if resp.StatusCode != 200 {
			// Handle specific error cases
			switch resp.StatusCode {
			case 400:
				// Bad Request - usually means expired or invalid credentials
				return fmt.Errorf("AWS federation service rejected the credentials (status 400)\n\nThis usually means your credentials are expired or invalid.\nPlease try:\n  awsm sso login <session-name>\n  aws sso login --sso-session <session-name>\n\nIf the problem persists, check your profile configuration with:\n  awsm profile list --detailed")
			case 403:
				// Forbidden - usually means insufficient permissions
				return fmt.Errorf("AWS federation service denied access (status 403)\n\nThis usually means your credentials don't have permission to access the console.\nPlease check your IAM permissions or try a different profile.")
			case 500, 502, 503, 504:
				// Server errors - AWS service issues
				return fmt.Errorf("AWS federation service is experiencing issues (status %d)\n\nPlease try again in a few minutes. If the problem persists, check AWS service status.", resp.StatusCode)
			default:
				// Other errors - show the full response for debugging
				if len(body) > 1000 {
					// Truncate very long responses (like HTML error pages)
					return fmt.Errorf("federation service returned status %d\n\nResponse (truncated): %s...\n\nPlease try refreshing your credentials:\n  awsm sso login <session-name>", resp.StatusCode, string(body[:1000]))
				}
				return fmt.Errorf("federation service returned status %d: %s\n\nPlease try refreshing your credentials:\n  awsm sso login <session-name>", resp.StatusCode, string(body))
			}
		}
		var tokenResp struct {
			SigninToken string `json:"SigninToken"`
		}
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return fmt.Errorf("failed to parse sign-in token response: %w. Response was: %s", err, string(body))
		}
		if tokenResp.SigninToken == "" {
			return fmt.Errorf("sign-in token not found in response. Response was: %s", string(body))
		}
		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = awsCfg.Region
		}
		if region == "" {
			region = "us-east-1"
			util.WarnColor.Fprintln(os.Stderr, "No region found, defaulting to us-east-1")
		}
		destination := fmt.Sprintf("https://%s.console.aws.amazon.com/console/home?region=%s", region, region)
		loginURL := fmt.Sprintf("https://signin.aws.amazon.com/federation?Action=login&Issuer=awsm&Destination=%s&SigninToken=%s", url.QueryEscape(destination), url.QueryEscape(tokenResp.SigninToken))

		if dontOpenBrowser {
			fmt.Println(loginURL)
		} else {
			// If --firefox-container is used, get the current AWS profile name
			var firefoxContainer string
			if useFirefox {
				firefoxContainer = os.Getenv("AWS_PROFILE")
				if firefoxContainer == "" {
					// Fallback to checking credentials file
					firefoxContainer = aws.GetCurrentProfileName()
					if firefoxContainer == "" {
						return fmt.Errorf("no AWS profile set. Please run 'awsm profile set <profile-name>' first")
					}
				}
			}

			if err := browser.OpenURL(loginURL, chromeProfile, firefoxContainer); err != nil {
				fmt.Fprintln(os.Stderr, "Could not open browser automatically. Please copy this URL:")
				fmt.Println(loginURL)
				return fmt.Errorf("could not open browser: %w", err)
			}
		}

		return nil
	},
}

func init() {
	consoleCmd.Flags().BoolVarP(&dontOpenBrowser, "no-open", "n", false, "Don't open the browser, just print the URL")
	consoleCmd.Flags().BoolVarP(&useFirefox, "firefox-container", "f", false, "Open in Firefox using a container named after the AWS profile")
	consoleCmd.Flags().StringVarP(&chromeProfile, "chrome-profile", "c", "", "Specify a Chrome profile alias or directory name (e.g., 'work')")
	rootCmd.AddCommand(consoleCmd)
}
