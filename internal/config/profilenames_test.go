package config

import (
	"reflect"
	"testing"
)

func names(profiles []GeneratedProfile) []string {
	out := make([]string, len(profiles))
	for i, p := range profiles {
		out[i] = p.Name
	}
	return out
}

func TestResolveProfileNameCollisionsLeavesUniqueNamesAlone(t *testing.T) {
	in := []GeneratedProfile{
		{Name: "s-alpha-admin", AccountID: "111111111111", RoleName: "Admin"},
		{Name: "s-beta-admin", AccountID: "222222222222", RoleName: "Admin"},
	}
	out, renames := ResolveProfileNameCollisions(in)

	if len(renames) != 0 {
		t.Errorf("renames = %v, want none", renames)
	}
	if got, want := names(out), []string{"s-alpha-admin", "s-beta-admin"}; !reflect.DeepEqual(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}

// "Foo Bar" and "Foo_Bar" both clean to "foo-bar". Merging them used to make
// one of the two accounts disappear from the config without a word.
func TestResolveProfileNameCollisionsKeepsBothAccounts(t *testing.T) {
	in := []GeneratedProfile{
		{Name: "s-foo-bar-admin", AccountID: "222222222222", RoleName: "Admin"},
		{Name: "s-foo-bar-admin", AccountID: "111111111111", RoleName: "Admin"},
	}
	out, renames := ResolveProfileNameCollisions(in)

	if len(renames) != 2 {
		t.Errorf("renames = %v, want 2 entries", renames)
	}
	got := names(out)
	want := []string{"s-foo-bar-admin-222222222222", "s-foo-bar-admin-111111111111"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
	seen := map[string]bool{}
	for _, n := range got {
		if seen[n] {
			t.Errorf("duplicate name %q survived", n)
		}
		seen[n] = true
	}
}

// Two roles in the same account can also clean to the same name, and then the
// account id is not enough to tell them apart.
func TestResolveProfileNameCollisionsWithinOneAccount(t *testing.T) {
	in := []GeneratedProfile{
		{Name: "s-acct-power-user", AccountID: "111111111111", RoleName: "Power_User"},
		{Name: "s-acct-power-user", AccountID: "111111111111", RoleName: "Power.User"},
	}
	out, _ := ResolveProfileNameCollisions(in)

	got := names(out)
	want := []string{"s-acct-power-user-111111111111-2", "s-acct-power-user-111111111111"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}

// The AWS API can return accounts and roles in any order; the resulting names
// must not depend on it, or every sync would rewrite the config differently.
func TestResolveProfileNameCollisionsIsOrderIndependent(t *testing.T) {
	a := GeneratedProfile{Name: "s-x-admin", AccountID: "111111111111", RoleName: "Admin"}
	b := GeneratedProfile{Name: "s-x-admin", AccountID: "222222222222", RoleName: "Admin"}
	c := GeneratedProfile{Name: "s-y-admin", AccountID: "333333333333", RoleName: "Admin"}

	forward, _ := ResolveProfileNameCollisions([]GeneratedProfile{a, b, c})
	reverse, _ := ResolveProfileNameCollisions([]GeneratedProfile{c, b, a})

	mapping := func(profiles []GeneratedProfile) map[string]string {
		m := map[string]string{}
		for _, p := range profiles {
			m[p.AccountID+"/"+p.RoleName] = p.Name
		}
		return m
	}
	if !reflect.DeepEqual(mapping(forward), mapping(reverse)) {
		t.Errorf("name assignment depends on input order:\n%v\n%v", mapping(forward), mapping(reverse))
	}
}

// A disambiguated name must never take a name that is already in use.
func TestResolveProfileNameCollisionsAvoidsExistingNames(t *testing.T) {
	in := []GeneratedProfile{
		{Name: "s-x", AccountID: "111111111111", RoleName: "Admin"},
		{Name: "s-x", AccountID: "222222222222", RoleName: "Admin"},
		{Name: "s-x-111111111111", AccountID: "333333333333", RoleName: "Admin"},
	}
	out, _ := ResolveProfileNameCollisions(in)

	seen := map[string]bool{}
	for _, p := range out {
		if seen[p.Name] {
			t.Errorf("duplicate name %q:\n%v", p.Name, names(out))
		}
		seen[p.Name] = true
	}
}

func TestResolveProfileNameCollisionsOnEmptyInput(t *testing.T) {
	out, renames := ResolveProfileNameCollisions(nil)
	if len(out) != 0 || len(renames) != 0 {
		t.Errorf("out = %v, renames = %v", out, renames)
	}
}
