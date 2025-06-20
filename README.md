# AWSM - AWS Manager

A powerful CLI tool to simplify working with AWS profiles, credentials, and sessions.

## Features

- **Profile Management**: Easily switch between AWS profiles with interactive selection
- **SSO Support**: Complete AWS SSO (IAM Identity Center) integration
- **MFA Support**: Streamlined MFA token handling for IAM profiles
- **Auto-refresh**: Automatic credential refresh when needed
- **Console Access**: Open the AWS console in your browser with proper credentials
- **Region Management**: Easily switch between AWS regions
- **Browser Integration**: Open the console in specific Chrome profiles or Firefox containers
- **Shell Completion**: Full autocompletion support for bash, zsh, fish, and PowerShell
- **Interactive UI**: Beautiful terminal interface with responsive design

## Installation

### From Releases

Download the latest release for your platform from the [Releases page](https://github.com/AleG03/awsm/releases).

### From Source

```bash
git clone https://github.com/AleG03/awsm.git
cd awsm
go build -o awsm .
```

## Usage

### Profile Management

```bash
# List all profiles
awsm profile list

# List profiles with detailed information
awsm profile list --detailed

# Set active profile
awsm profile set my-profile

# Login to SSO profile and set as active
awsm profile login my-sso-profile

# Change default region for a profile
awsm profile change-default-region my-profile eu-central-1
```

### Interactive Profile Selection

```bash
# Interactive profile selector with arrow keys
awsm select
```

### SSO Management

```bash
# Add SSO session to config and automatically generate profiles
awsm sso add my-session https://d-123456789.awsapps.com/start/ us-east-1

# Login to SSO session
awsm sso login my-sso-session

# Generate profiles from SSO (discovers all accounts/roles)
awsm sso generate my-sso-session

# List all SSO Sessions
awsm sso list

# List SSO Sessions with detailed information
awsm sso list --detailed
```

### Credential Management

```bash
# Refresh credentials for current or specified profile
awsm refresh [profile-name]

# Clear all credentials from default profile
awsm clear
```

### Console Access

```bash
# Open AWS console in default browser
awsm console

# Open in Firefox container (uses profile name as container)
awsm console --firefox-container

# Open in specific Chrome profile
awsm console --chrome-profile work

# Just print the URL without opening browser
awsm console --no-open
```

### Region Management

```bash
# List all available AWS regions
awsm region list

# Set region for default profile
awsm region set us-west-2
```

### Shell Completion

```bash
# Generate completion script for your shell
awsm completion bash   # or zsh, fish, powershell

# Install bash completion (Linux)
awsm completion bash > /etc/bash_completion.d/awsm

# Install zsh completion (create directory first if needed)
mkdir -p ~/.zsh/completions
awsm completion zsh > ~/.zsh/completions/_awsm
# Add to your ~/.zshrc (if not already present)
echo 'fpath=(~/.zsh/completions $fpath)' >> ~/.zshrc
echo 'autoload -U compinit && compinit' >> ~/.zshrc

# Alternative: Use system zsh completion directory (macOS)
sudo mkdir -p /usr/local/share/zsh/site-functions
sudo awsm completion zsh > /usr/local/share/zsh/site-functions/_awsm
```

### Software Update

```bash
# Update to latest version
sudo awsm update
```

## Configuration

AWSM uses the standard AWS configuration files:

- `~/.aws/config` - Profile configurations
- `~/.aws/credentials` - Credentials storage

Additional AWSM-specific configuration can be placed in `~/.config/awsm/config.toml`:

```toml
[chrome_profiles]
work = "Profile 1"
personal = "Profile 2"
```

### Profile Types

AWSM supports three types of AWS profiles:

- **SSO Profiles**: Use AWS IAM Identity Center for authentication
- **IAM Profiles**: Use IAM roles with MFA for authentication
- **Static Profiles**: Use long-term access keys (not recommended for production)

### Example SSO Session Configuration

```ini
[sso-session my-company]
sso_start_url = https://d-123456789.awsapps.com/start/
sso_region = us-east-1
sso_registration_scopes = sso:account:access
```

### Example Profile Configuration

```ini
[profile my-company-admin]
sso_session = my-company
sso_account_id = 123456789012
sso_role_name = AdministratorAccess
region = us-east-1
```

## License

This project is licensed under the Business Source License 1.1.

- **Non-Commercial Use**: Free for personal, educational, and non-commercial use
- **Commercial Use**: Prohibited until 2028-01-01
- **After 2028-01-01**: Available under Apache License 2.0

See the [LICENSE](LICENSE) file for details.

For commercial licensing before 2028, please contact gc.ale03@gmail.com.

## Key Features in Detail

### Interactive Profile Selector
- Responsive terminal UI that adapts to your terminal size
- Color-coded profile types (SSO, IAM, Static)
- Shows account IDs, regions, and active status
- Filter and search capabilities

### Smart Credential Management
- Automatic credential refresh for expired sessions
- Preserves profile context when switching regions
- Tracks active profile in default credentials
- Handles both temporary and static credentials

### Browser Integration
- Generates federated sign-in URLs for AWS Console
- Chrome profile support with custom aliases
- Firefox Multi-Account Container integration
- Automatic region detection for console URLs

### Shell Completion
- Tab completion for all commands and flags
- Profile name completion for relevant commands
- Works with bash, zsh, fish, and PowerShell
- Easy installation with generated scripts

## Acknowledgments

- [AWS SDK for Go](https://github.com/aws/aws-sdk-go-v2)
- [Cobra](https://github.com/spf13/cobra)
- [Viper](https://github.com/spf13/viper)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss)