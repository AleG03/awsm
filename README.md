# AWSM - AWS Manager

A powerful CLI tool to simplify working with AWS profiles, credentials, and sessions.

## Features

- **Profile Management**: Easily switch between AWS profiles with interactive selection
- **SSO Support**: Complete AWS SSO (IAM Identity Center) integration with automatic profile generation. Signing in is done by awsm itself over the OIDC device flow, so the AWS CLI is not needed for it
- **MFA Support**: Streamlined MFA token handling for IAM profiles
- **Identity Inspection**: `awsm whoami` shows account, ARN, region and credentials TTL at a glance
- **Run Commands Scoped to a Profile**: `awsm run` injects AWS credentials into a child process without polluting your shell
- **Shell Prompt Integration**: `awsm prompt` produces a fast, offline status string for use, including a warning when the renewal daemon needs you
- **Diagnostics**: `awsm doctor` audits your local setup (config files, permissions, external tools, browsers)
- **Smart Conflict Resolution**: Intelligent handling of profile name conflicts during creation
- **Console Access**: Open the AWS console in your browser with proper credentials
- **Connect & Port Forwarding**: Connect to EC2 instances via SSM and setup advanced port forwarding (RDS, remote hosts)
- **Region Management**: Easily switch between AWS regions
- **Search & Discovery**: Powerful search across profiles, account IDs, and SSO sessions with partial matching
- **Browser Integration**: Open the console in specific Chrome profiles or Firefox containers
- **Firefox Extension**: First-party [`AWSM Container Opener`](https://addons.mozilla.org/firefox/addon/awsm-container-opener/) add-on published on AMO; `awsm extension install` opens the listing for 1-click permanent install with auto-updates
- **Automatic Credential Renewal**: `awsm daemon` keeps the active profile's credentials from expiring, with no resident process
- **Shell Completion**: Full autocompletion support for bash, zsh, fish, and PowerShell
- **Interactive UI**: Beautiful terminal interface with responsive design
- **Import/Export**: Backup and restore your AWS configuration

## Installation

### Requirements

A single binary, and nothing else for everyday use. In particular **the AWS CLI
is not required**: awsm talks to AWS through the SDK, and signs in to SSO itself
over the OIDC device authorization flow.

Two commands do reach for something external:

| | |
|---|---|
| `awsm connect` | the AWS CLI and [`session-manager-plugin`](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html). SSM sessions are carried by that plugin, which has no substitute |
| `awsm console` | a browser |

`awsm doctor` reports what it finds, and what it does not.

Note that awsm's job is to furnish credentials *to* the AWS CLI and everything
else that reads `~/.aws`. Not requiring it is not a reason to remove it.

### From Releases

Download the latest release for your platform from the [Releases page](https://github.com/AleG03/awsm/releases).
Copy the binary to your PATH and run `awsm` from anywhere.
For MacOS users, you can also copy the binary into /usr/local/bin and run from everyone.
For arch linux users, the binary is available in the AUR.

### Using Homebrew

If you have Homebrew installed, you can install awsm with:

```bash
brew tap aleg03/awsm
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

### Identity & Status

```bash
# Show the active identity (account, ARN, region, credentials TTL)
awsm whoami

# Inspect a specific profile without switching to it
awsm whoami --profile my-profile

# Machine-readable output for scripts
awsm whoami --json

# Skip the sts:GetCallerIdentity call (offline / fast)
awsm whoami --no-call
```

The TTL is color-coded: green when there's plenty of time left, amber under 30
minutes, red under 5 minutes or already expired. SSO profiles use the SSO
access-token cache to compute the remaining lifetime.

### Run Commands With Profile Credentials

`awsm run` resolves credentials for the chosen profile (auto-prompting MFA and
signing in to SSO if needed) and executes the given command with
`AWS_*` environment variables injected. The parent shell is **not** modified.

```bash
# Use the active profile
awsm run -- aws s3 ls

# Pin a specific profile / region for the child process
awsm run --profile prod -- terraform plan
awsm run --profile dev --region eu-west-1 -- env | grep AWS_
```

Anything before `--` is parsed as `awsm` flags; everything after is the command
to execute. The child process's exit code is propagated.

### Shell Prompt Integration

`awsm prompt` prints a compact one-line status string designed to be embedded
in your shell prompt. It is fast and offline (no AWS API calls).

```bash
# Default format: "{icon} {profile}:{region} {ttl}{blocked}"
awsm prompt

# Custom format
awsm prompt --format '{profile}@{region} ({ttl})'
awsm prompt --format '☁ {profile}'

# Disable ANSI colors
awsm prompt --no-color

# Print nothing if no profile is active (instead of "(no profile)")
awsm prompt --empty-on-none
```

Available placeholders: `{profile}`, `{region}`, `{type}`, `{ttl}`, `{account}`,
`{icon}`, `{blocked}`.

`{blocked}` is empty unless the renewal daemon is stuck on something only you
can resolve, in which case it becomes ` ⚠ MFA` or ` ⚠ SSO`:

```
🔐 prod-admin:eu-west-1 52m10s          # healthy
🔐 prod-admin:eu-west-1 2m09s ⚠ MFA     # needs an MFA code
☁ client-admin:eu-central-1 4m32s ⚠ SSO # needs an SSO login
```

A notification is a moment; this is the reminder that stays, and it appears
where you are about to use AWS. It reads the daemon's own state file, so it
never disagrees with what the notification said, and it clears itself as soon
as a renewal succeeds. See [Automatic Credential Renewal](#automatic-credential-renewal).

**Zsh** (`~/.zshrc`) — on the right-hand side, where it does not move the
cursor as you type:

```bash
setopt PROMPT_SUBST
RPROMPT='$(awsm prompt --empty-on-none)'
```

The single quotes matter: they keep `$(...)` from being evaluated once when the
file loads instead of on every prompt.

**Starship example** (`~/.config/starship.toml`):

```toml
[custom.awsm]
command = "awsm prompt --no-color"
when = "awsm profile current"
format = "[$output]($style) "
style = "bold cyan"
```

**Bash PS1 example**:

```bash
PS1='$(awsm prompt --empty-on-none) \w $ '
```

Cost: one invocation per prompt, measured at about 6 ms. Nothing runs in the
background — the shell calls `awsm`, which reads two local files and exits.

None of this is required. The daemon renews credentials whether or not the
prompt shows anything; what you lose without it is seeing a stuck renewal
between the notification and the moment you act on it.

### Diagnostics

`awsm doctor` audits the local environment and prints a categorized report
(versions, AWS files and permissions, external tools, awsm config, profiles
and cache, browsers). It exits non-zero if any check is `FAIL`.

```bash
awsm doctor

# Machine-readable output (great for bug reports)
awsm doctor --json
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

For Firefox, AWSM relies on the first-party **AWSM Container Opener**
extension — published on [addons.mozilla.org](https://addons.mozilla.org/firefox/addon/awsm-container-opener/)
and maintained in a [dedicated repository](https://github.com/AleG03/awsm-firefox-container).
It handles the `ext+container:` protocol used by `awsm console --firefox-container`.

**Step 1: Install the extension**

```bash
# Opens the AMO listing in your browser — click "Add to Firefox"
awsm extension install

# Show extension info and links
awsm extension status
```

The extension installs permanently and auto-updates via AMO.

**Step 2: Use Firefox Containers**

```bash
# Open console in Firefox container (uses profile name as container name)
awsm console --firefox-container

# This will attempt to open the AWS console in a Firefox container
# with the same name as your current AWS profile
```

**Note**: The container name will match your AWS profile name. If you have a profile named `work-production`, AWSM will try to open the console in a Firefox container named `work-production`.

**Deterministic colors and icons**: The AWSM Container Opener derives each
container's color and icon from a hash of its name, so the same AWS profile
always gets the same visual identity. Existing containers are never
re-colored. You can toggle URL-supplied color/icon hints in the extension's
options page if you want the legacy random behavior.

#### Zen Container Integration

Same as Firefox, AWSM can open the AWS console in Zen browser containers.

### Connect & Port Forwarding

AWSM provides a `connect` command (aliased as `ssm` or `con`) to connect to your EC2 instances via AWS Systems Manager (SSM) Session Manager without needing SSH keys or open inbound ports.

It also supports advanced port forwarding capabilities, including forwarding to remote hosts (like RDS databases) through an EC2 bastion.

#### Basic Usage

```bash
# Connect to an instance (opens interactive shell)
awsm connect i-0123456789abcdef0

# Select an instance interactively from a list of running instances
awsm connect
```

#### Port Forwarding

```bash
# Forward a remote port to your local machine
awsm connect i-0123456789abcdef0 --port-forwarding --remote-port 80 --local-port 8080

# Forward to a remote host (e.g., RDS) via the instance
awsm connect i-0123456789abcdef0 -p -r 5432 -H my-db.cluster-c1gqquowm81q.eu-west-1.rds.amazonaws.com
```

#### Configuration Files

You can define your connection parameters in a JSON file for easy reuse:

```bash
# connect.json
{
  "instance_id": "i-0d7211b09c641f135",
  "remote_host": "database-1.cluster-xxx.rds.amazonaws.com",
  "remote_port": 5432,
  "local_port": 5432
}

# Use the config file
awsm connect -f connect.json
```

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

### Automatic Credential Renewal

Temporary credentials expire after an hour. `awsm daemon` renews the active
profile's credentials before that happens.

```bash
# Start renewing, and keep doing it at every login
awsm daemon enable

# Also open the SSO login page when the session can no longer be refreshed
awsm daemon enable --auto-login

awsm daemon status     # is it running, and what did it last do
awsm daemon logs       # recent activity
awsm daemon disable    # stop and remove the scheduled job
```

**Nothing stays resident.** The system scheduler runs `awsm` briefly once a
minute, it checks how much life the credentials have left and exits: a cycle
that finds nothing to do costs a few file reads and no network traffic. On
macOS, 60 consecutive idle cycles — an hour's worth — measured 0.23 s of CPU in
total, and nothing is held between them. A sleeping daemon would instead hold
about 13 MB of memory around the clock to do the same job.

Credentials are renewed once less than ten minutes remain. That margin is a
retry budget rather than a safety cushion: everything under it is time available
to recover from a failed attempt before anything actually expires. Already
expired credentials — the normal state after the machine has been asleep — are
renewed at the next cycle rather than ignored.

It uses a LaunchAgent on macOS, a systemd user timer on Linux, and a Scheduled
Task on Windows. All three start it at login, and `disable` removes the job
entirely.

#### What it cannot renew on its own

Two situations need you, and both are detected before any network call, then
reported **once** rather than retried every minute. The notification names the
command to run; if you add `{blocked}` to your shell prompt (see
[Shell Prompt Integration](#shell-prompt-integration)) the warning also stays
visible until it is resolved, instead of passing with the notification:

- **Profiles requiring an MFA code.** Covered while a cached MFA session lasts
  (see below); after that the code has to be typed again.
- **SSO sessions that can no longer be refreshed.** Ordinary SSO token expiry is
  handled silently: the daemon renews the token half an hour before it lapses,
  using the refresh token stored at login. Access tokens are
  short — an hour against one Identity Center tested here — so without this
  you are logged out roughly hourly. The browser is needed only once the
  refresh token itself stops being accepted, which the daemon recognises by the
  `InvalidGrantException` that comes back and reports once rather than retrying.

### MFA sessions

A profile with `mfa_serial` used to ask for a code on every renewal. awsm now
obtains a session token once, valid for up to 36 hours, and derives role
credentials from it without asking again. Those credentials still carry the MFA
claim, so trust policies that require MFA are satisfied.

This is what makes MFA profiles renewable unattended. Two consequences worth
knowing:

- `~/.awsm/cache/` now holds a credential that stays valid for up to 36 hours.
  The file is owner-only, and `awsm profile set` re-acquires it once it lapses.
  `awsm daemon` never creates one, since obtaining it needs a code.
- For a profile that uses `mfa_serial` without `role_arn`, `awsm whoami` will
  report a TTL of up to 36 hours where it used to say one hour. That is the
  session's real lifetime; it did not change, it was simply being re-acquired
  every hour before.

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
- **Commercial Use**: Prohibited until 2030-01-01
- **After 2030-01-01**: Available under Apache License 2.0

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

### Connect & Port Forwarding
- **Interactive Selection**: Beautiful selector for running EC2 instances with state and IP information
- **Advanced Forwarding**: Support for `AWS-StartPortForwardingSessionToRemoteHost` and `AWS-StartPortForwardingSession`
- **Configurable**: Support for JSON configuration files for complex tunnel setups
- **Self-Healing**: Automatically detects missing `session-manager-plugin` and provides installation links
- **Security First**: Proactive warnings when trying to bind to privileged ports (< 1024) without root

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