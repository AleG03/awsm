package config

import (
	"fmt"
	"sort"
	"strconv"
)

// GeneratedProfile is one profile awsm is about to write for an SSO session.
type GeneratedProfile struct {
	Name       string
	SSOSession string
	AccountID  string
	RoleName   string
	Region     string
}

// ResolveProfileNameCollisions makes every profile name unique and reports the
// renames it had to make.
//
// Profile names are derived from the account and role names with every
// character outside [a-zA-Z0-9-] replaced by "-", so accounts called "Foo Bar",
// "Foo_Bar" and "Foo.Bar" all collapse onto the same name. Two sections with
// the same header make the AWS CLI reject the whole config file, and merging
// them (which is what awsm used to end up doing) silently dropped one of the
// accounts. Colliding names get the account id appended instead, and a numeric
// suffix if that is still not enough.
//
// The result only depends on the profiles themselves, never on the order the
// AWS API returned them, so repeated runs produce the same config.
func ResolveProfileNameCollisions(profiles []GeneratedProfile) ([]GeneratedProfile, []string) {
	groups := make(map[string][]int)
	for i, p := range profiles {
		groups[p.Name] = append(groups[p.Name], i)
	}

	// Names that are already unique are reserved before any renaming, so a
	// disambiguated name can never steal one of them.
	taken := make(map[string]bool)
	var collided []string
	for name, idx := range groups {
		if len(idx) == 1 {
			taken[name] = true
		} else {
			collided = append(collided, name)
		}
	}
	sort.Strings(collided)

	resolved := make([]GeneratedProfile, len(profiles))
	copy(resolved, profiles)

	var renames []string
	for _, name := range collided {
		idx := groups[name]
		sort.Slice(idx, func(a, b int) bool {
			pa, pb := profiles[idx[a]], profiles[idx[b]]
			if pa.AccountID != pb.AccountID {
				return pa.AccountID < pb.AccountID
			}
			return pa.RoleName < pb.RoleName
		})
		for _, i := range idx {
			candidate := fmt.Sprintf("%s-%s", name, profiles[i].AccountID)
			for n := 2; taken[candidate]; n++ {
				candidate = fmt.Sprintf("%s-%s-%s", name, profiles[i].AccountID, strconv.Itoa(n))
			}
			taken[candidate] = true
			resolved[i].Name = candidate
			renames = append(renames, fmt.Sprintf("%s -> %s (account %s, role %s)",
				name, candidate, profiles[i].AccountID, profiles[i].RoleName))
		}
	}

	return resolved, renames
}
