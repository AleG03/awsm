package aws

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"awsm/internal/awsini"
)

// Cache v2 separates credential kinds and hashes names so profile names cannot
// collide with another namespace or escape the cache directory. Legacy entries
// are deliberately ignored: they have no configuration identity to validate.
func credentialCachePath(kind, profile string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%x.json", sha256.Sum256([]byte(profile)))
	return filepath.Join(home, ".awsm", "cache", "v2", kind, name), nil
}

type credentialCacheEntry struct {
	TempCredentials
	Fingerprint string `json:"fingerprint"`
}

// profileFingerprint includes the profile's source chain and SSO session, plus
// both AWS file paths. Unrelated profiles and active-default writes do not
// invalidate a named profile's cache. Only a digest of configuration is stored.
func profileFingerprint(profile string) (string, error) {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return "", err
	}
	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return "", err
	}
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return "", err
	}
	credentialsPath, err = filepath.Abs(credentialsPath)
	if err != nil {
		return "", err
	}
	cfg, err := awsini.LoadOrEmpty(configPath)
	if err != nil {
		return "", err
	}
	creds, err := awsini.LoadOrEmpty(credentialsPath)
	if err != nil {
		return "", err
	}
	sections := map[string]map[string]string{}
	visited := map[string]bool{}
	var collect func(string)
	collect = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		merged := map[string]string{}
		if section, err := getProfileSection(cfg, name); err == nil {
			sections["config:"+name] = section.KeysHash()
			for k, v := range section.KeysHash() {
				merged[k] = v
			}
		}
		if section, err := creds.GetSection(name); err == nil {
			sections["credentials:"+name] = section.KeysHash()
			for k, v := range section.KeysHash() {
				merged[k] = v
			}
		}
		if source := merged["source_profile"]; source != "" {
			collect(source)
		}
		if session := merged["sso_session"]; session != "" {
			if section, err := cfg.GetSection("sso-session " + session); err == nil {
				sections["session:"+session] = section.KeysHash()
			}
		}
	}
	collect(profile)
	raw, err := json.Marshal(struct {
		Profile, ConfigPath, CredentialsPath string
		Sections                             map[string]map[string]string
	}{profile, configPath, credentialsPath, sections})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw)), nil
}

func readCredentialCache(path, profile string) *TempCredentials {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entry credentialCacheEntry
	if json.Unmarshal(raw, &entry) != nil {
		return nil
	}
	fingerprint, err := profileFingerprint(profile)
	if err != nil || entry.Fingerprint != fingerprint {
		return nil
	}
	return &entry.TempCredentials
}

func writeCredentialCache(path, profile string, creds *TempCredentials) {
	fingerprint, err := profileFingerprint(profile)
	if err != nil {
		return
	}
	writeCredentialCacheWithFingerprint(path, fingerprint, creds)
}

func writeCredentialCacheWithFingerprint(path, fingerprint string, creds *TempCredentials) {
	raw, err := json.Marshal(credentialCacheEntry{*creds, fingerprint})
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}
	_ = awsini.WritePrivateFile(path, raw)
}

// ActiveCredentialsRevision is read before taking a daemon snapshot. Comparing
// it under the write lock also detects a clear or a new session for the same
// profile, including a switch away and back while AWS was responding.
func ActiveCredentialsRevision() (string, error) {
	path, err := GetAWSCredentialsPath()
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw)), nil
}
