# AWSM - AWS Manager

A powerful CLI tool to simplify working with AWS profiles, credentials, and sessions.

## Features

- **Profile Management**: Easily switch between AWS profiles
- **SSO Support**: Seamless integration with AWS SSO
- **MFA Support**: Streamlined MFA token handling
- **Auto-refresh**: Automatic credential refresh when needed
- **Console Access**: Open the AWS console in your browser with proper credentials
- **Region Management**: Easily switch between AWS regions
- **Browser Integration**: Open the console in specific Chrome profiles or Firefox containers

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

# Set active profile
awsm profile set my-profile

# Show detailed profile info
awsm profile list --detailed
```

### Fancy Profile Management

```bash
# List all profiles and select one using arrows
awsm select
```

### SSO Management

```bash
# Login to SSO
awsm sso login my-sso-profile

# Generate profiles from SSO
awsm sso generate my-sso-session
```

### Credential Management

```bash
# Refresh credentials
awsm refresh [profile-name]

# Clear credentials
awsm clear [profile-name]
```

### Console Access

```bash
# Open AWS console in browser
awsm console

# Open in Firefox container
awsm console --firefox-container

# Open in Chrome profile
awsm console --chrome-profile work
```

### Region Management

```bash
# List available regions
awsm region list

# Set region
awsm region set us-west-2
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

## License

This project is licensed under the Business Source License 1.1.

- **Non-Commercial Use**: Free for personal, educational, and non-commercial use
- **Commercial Use**: Prohibited until 2028-01-01
- **After 2028-01-01**: Available under Apache License 2.0

See the [LICENSE](LICENSE) file for details.

For commercial licensing before 2028, please contact gc.ale03@gmail.com.

## Acknowledgments

- [AWS SDK for Go](https://github.com/aws/aws-sdk-go-v2)
- [Cobra](https://github.com/spf13/cobra)
- [Viper](https://github.com/spf13/viper)