# AWSM - AWS Manager

A powerful CLI tool to simplify working with AWS profiles, credentials, and sessions.

## Features

- **Profile Management**: Easily switch between AWS profiles with interactive selection
- **SSO Support**: Complete AWS SSO (IAM Identity Center) integration with automatic profile generation
- **MFA Support**: Streamlined MFA token handling for IAM profiles
- **Smart Conflict Resolution**: Intelligent handling of profile name conflicts during creation
- **Console Access**: Open the AWS console in your browser with proper credentials
- **Region Management**: Easily switch between AWS regions
- **Search & Discovery**: Powerful search across profiles, account IDs, and SSO sessions with partial matching
- **Browser Integration**: Open the console in specific Chrome profiles or Firefox containers
- **Shell Completion**: Full autocompletion support for bash, zsh, fish, and PowerShell
- **Interactive UI**: Beautiful terminal interface with responsive design
- **Import/Export**: Backup and restore your AWS configuration

## Installation

### From Releases

Download the latest release for your platform from the [Releases page](https://github.com/AleG03/awsm/releases).
Copy the binary to your PATH and run `awsm` from anywhere.
For MacOS users, you can also copy the binary into /usr/local/bin and run from everyone.
For arch linux users, the binary is available in the AUR.

### Using Homebrew

If you have Homebrew installed, you can install awsm with:

```bash
brew tap aleg03/awsm-tap
brew install awsm
```

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

# Login to SSO profile and set as active
awsm profile set my-profile

# Change default region for a profile
awsm profile change-default-region my-profile eu-central-1

# Add new profiles
awsm profile add iam-user my-user        # Add IAM user profile with access keys
awsm profile add iam-role my-role        # Add IAM role with assumption

# Edit profiles
awsm profile edit my-profile             # Edit existing profile interactively

# Delete profiles
awsm profile delete my-profile           # Delete single profile
awsm profile delete --all-sso my-session # Delete all profiles for SSO session
awsm profile delete --force my-profile   # Delete without confirmation
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

# Delete SSO session and all associated profiles
awsm sso delete my-session               # Interactive deletion
awsm sso delete --force my-session       # Delete without confirmation
```

### Credential Management

# Clear all credentials from default profile
awsm clear

# Export/Import configurations
awsm export [output-file]               # Export all profiles and SSO sessions
awsm import <export-file>                # Import from export file
awsm import --force <export-file>        # Import without confirmation
```

### Console Access

AWSM can open the AWS console in your browser with proper credentials. It supports both Chrome profiles and Firefox containers for better organization.

#### Basic Usage

```bash
# Open AWS console in default browser
awsm console

# Just print the URL without opening browser
awsm console --no-open
```

#### Chrome Profile Integration

To use Chrome profiles with AWSM, you need to configure profile mappings in your AWSM configuration file.

**Step 1: Find Your Chrome Profile Numbers**

Chrome stores profiles with numeric identifiers. To find your profile numbers:

1. Open Chrome and go to `chrome://version/`
2. Look for the "Profile Path" - it will show something like:
   - `Profile 1` (for the first additional profile)
   - `Profile 2` (for the second additional profile)
   - `Default` (for the default profile)

Alternatively, you can check your Chrome profile directory:
- **macOS**: `~/Library/Application Support/Google/Chrome/`
- **Linux**: `~/.config/google-chrome/`
- **Windows**: `%LOCALAPPDATA%\Google\Chrome\User Data\`

**Step 2: Configure Profile Mappings**

Create or edit `~/.config/awsm/config.toml` and add your Chrome profile mappings:

```toml
[chrome_profiles]
work = "Profile 1"
personal = "Profile 2"
default = "Default"
company = "Profile 3"
```

**Step 3: Use Chrome Profiles**

```bash
# Open console in specific Chrome profile
awsm console --chrome-profile work
awsm console --chrome-profile personal
awsm console --chrome-profile default
```

#### Firefox Container Integration

For Firefox, AWSM uses the "Open external links in a container" extension to open AWS console links in specific containers.

**Step 1: Install the Extension**

1. Install the [Open external links in a container](https://addons.mozilla.org/en-US/firefox/addon/open-url-in-container/) extension from Firefox Add-ons
2. The extension allows external links to be opened in specific Firefox containers

**Step 2: Use Firefox Containers**

```bash
# Open console in Firefox container (uses profile name as container name)
awsm console --firefox-container

# This will attempt to open the AWS console in a Firefox container
# with the same name as your current AWS profile
```

**Note**: The container name will match your AWS profile name. If you have a profile named `work-production`, AWSM will try to open the console in a Firefox container named `work-production`.

**Random Colors and Icons**: When AWSM creates a new Firefox container, it automatically assigns a random color and icon from Firefox's available options. This helps visually distinguish between different AWS profiles. Available colors include: blue, turquoise, green, yellow, orange, red, pink, and purple. Available icons include: fingerprint, briefcase, dollar, cart, circle, gift, vacation, food, fruit, pet, tree, and chill.


### Region Management

```bash
# List all available AWS regions
awsm region list

# Set region for default profile
awsm region set us-west-2
```

### Search and Discovery

```bash
# Search everything (profiles, account IDs, SSO sessions)
awsm search my-profile             # Find profiles containing 'my-profile'
awsm search 123456789012           # Find profiles with this account ID
awsm search 1234                   # Find profiles with partial account ID
awsm search test                # Find SSO sessions or profiles with 'test'

# Search specific types only
awsm search --account 8517          # Search only account IDs for '8517'
awsm search --profile prod          # Search only profile names for 'prod'
awsm search --sso my-session       # Search only SSO session names

# Case-sensitive search
awsm search --case-sensitive MyProfile
```

#### Installation

### Shell Completion

AWSM supports tab completion for commands, subcommands, flags, and profile names across multiple shells.

#### Bash

**Linux:**
```bash
# Install completion
awsm completion bash | sudo tee /etc/bash_completion.d/awsm

# Reload your shell
source ~/.bashrc
```

**macOS:**
```bash
# Install bash-completion if not already installed
brew install bash-completion

# Install AWSM completion
awsm completion bash > $(brew --prefix)/etc/bash_completion.d/awsm

# Reload your shell
source ~/.bash_profile
```

#### Zsh

**User-specific installation:**
```bash
# Create completions directory
mkdir -p ~/.zsh/completions

# Generate completion file
awsm completion zsh > ~/.zsh/completions/_awsm

# Add to ~/.zshrc (if not already present)
echo 'fpath=(~/.zsh/completions $fpath)' >> ~/.zshrc
echo 'autoload -U compinit && compinit' >> ~/.zshrc

# Reload your shell
source ~/.zshrc
```

**System-wide installation (macOS):**
```bash
# Install to system directory
sudo mkdir -p /usr/local/share/zsh/site-functions
sudo awsm completion zsh > /usr/local/share/zsh/site-functions/_awsm

# Reload your shell
source ~/.zshrc
```

#### Fish

```bash
# Create completions directory
mkdir -p ~/.config/fish/completions

# Generate completion file
awsm completion fish > ~/.config/fish/completions/awsm.fish

# Completions are automatically loaded (no restart needed)
```

**System-wide installation:**
```bash
# macOS with Homebrew Fish
sudo awsm completion fish > /usr/local/share/fish/vendor_completions.d/awsm.fish

# Linux systems
sudo awsm completion fish > /usr/share/fish/vendor_completions.d/awsm.fish
```

#### PowerShell

```powershell
# Create PowerShell profile directory if it doesn't exist
if (!(Test-Path -Path $PROFILE)) {
    New-Item -ItemType File -Path $PROFILE -Force
}

# Add completion to your PowerShell profile
awsm completion powershell >> $PROFILE

# Reload your profile
. $PROFILE
```

**Alternative method:**
```powershell
# Generate completion script
awsm completion powershell | Out-String | Invoke-Expression
```

#### Testing Completions

After installation, test your completions:

```bash
# Tab complete commands
awsm <TAB>

# Tab complete subcommands
awsm profile <TAB>

# Tab complete profile names
awsm profile set <TAB>

# Tab complete flags
awsm profile list --<TAB>
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
- **IAM User Profiles**: Use long-term access keys (not recommended for production)

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
- Preserves profile context when switching regions
- Tracks active profile in default credentials
- Handles both temporary and static credentials

### Advanced Search & Discovery
- **Universal Search**: Search profiles, account IDs, and SSO sessions simultaneously
- **Partial Matching**: Find accounts with partial IDs (e.g., `1234` finds `123456789012`)
- **Flexible Queries**: Search beginning, middle, or end of strings
- **Type-Specific Search**: Use `--account`, `--profile`, or `--sso` flags for targeted searches
- **Color-Coded Results**: Visual distinction between SSO, IAM, and Key profiles
- **Case Sensitivity**: Optional case-sensitive search with `--case-sensitive` flag

### Smart Conflict Resolution
- Detects existing profiles before creation
- Offers multiple resolution options:
  - Skip profile creation
  - Auto-rename with type suffix
  - Custom name input
  - Overwrite existing profile
- Consistent experience across all profile types

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

## Development

### Building from Source

```bash
git clone https://github.com/AleG03/awsm.git
cd awsm
go build -o awsm .
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/aws
```

### Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Acknowledgments

- [AWS SDK for Go](https://github.com/aws/aws-sdk-go-v2)
- [Cobra](https://github.com/spf13/cobra)
- [Viper](https://github.com/spf13/viper)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss)