package config

import (
	"reflect"
	"testing"
)

func unnormalizedConfig() OrganizationConfig {
	return OrganizationConfig{
		Organization: " acme ",
		Members: []OrganizationMemberSpec{
			{Username: " alice ", Role: " member "},
		},
		Invites: []InviteSpec{
			{Username: optionalString(" octocat "), Role: " direct_member ", TeamSlugs: []string{" platform "}},
		},
		Repositories: []RepositorySpec{
			{Name: " service ", Visibility: " private ", Topics: []string{" go "},
				Template: TemplateSpec{Owner: " acme ", Name: " base "}},
		},
		Teams: []TeamSpec{
			{Slug: " platform ", Name: " Platform ", Description: " desc ", Privacy: " closed ",
				Members:      []TeamMemberSpec{{Username: " alice ", Role: " maintainer "}},
				Repositories: []TeamRepositorySpec{{Name: " service ", Permission: " push "}}},
		},
	}
}

func TestNormalizeDesiredStateTrimsEveryLoadNormalizedField(t *testing.T) {
	t.Parallel()

	got := NormalizeDesiredState(unnormalizedConfig())

	if got.Organization != "acme" {
		t.Fatalf("organization: %q", got.Organization)
	}
	if got.Members[0].Username != "alice" || got.Members[0].Role != "member" {
		t.Fatalf("member: %#v", got.Members[0])
	}
	if got.Invites[0].Username.Value != "octocat" || got.Invites[0].Role != "direct_member" || got.Invites[0].TeamSlugs[0] != "platform" {
		t.Fatalf("invite: %#v", got.Invites[0])
	}
	repo := got.Repositories[0]
	if repo.Owner != "acme" || repo.Name != "service" || repo.Visibility != "private" || repo.Topics[0] != "go" {
		t.Fatalf("repository: %#v", repo)
	}
	if repo.Template.Owner != "acme" || repo.Template.Name != "base" {
		t.Fatalf("template: %#v", repo.Template)
	}
	team := got.Teams[0]
	if team.Slug != "platform" || team.Name != "Platform" || team.Description != "desc" || team.Privacy != "closed" {
		t.Fatalf("team: %#v", team)
	}
	if team.Members[0].Username != "alice" || team.Members[0].Role != "maintainer" {
		t.Fatalf("team member: %#v", team.Members[0])
	}
	if team.Repositories[0].Owner != "acme" || team.Repositories[0].Name != "service" || team.Repositories[0].Permission != "push" {
		t.Fatalf("team repository: %#v", team.Repositories[0])
	}
}

func TestNormalizeDesiredStateDoesNotMutateCaller(t *testing.T) {
	t.Parallel()

	original := unnormalizedConfig()
	snapshot := unnormalizedConfig()

	_ = NormalizeDesiredState(original)

	if !reflect.DeepEqual(original, snapshot) {
		t.Fatalf("NormalizeDesiredState mutated caller config:\n got %#v\nwant %#v", original, snapshot)
	}
}

func TestEncodeYAMLDoesNotMutateNestedSlices(t *testing.T) {
	t.Parallel()

	original := unnormalizedConfig()
	snapshot := unnormalizedConfig()

	if _, err := EncodeYAML(original); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, snapshot) {
		t.Fatalf("EncodeYAML mutated caller config:\n got %#v\nwant %#v", original, snapshot)
	}
}

func TestNormalizeDesiredStateDoesNotRepairInvalidValues(t *testing.T) {
	t.Parallel()

	cfg := OrganizationConfig{
		Organization: "acme",
		Repositories: []RepositorySpec{{Name: "service", Visibility: " not-a-visibility "}},
	}
	if report := Validate(NormalizeDesiredState(cfg)); report.Valid {
		t.Fatal("normalization must not turn an invalid enum into a valid config")
	}
}

// sliceBackingArrays walks v and collects the backing-array address of every
// slice it reaches, so two configs can be compared for shared storage.
func sliceBackingArrays(v reflect.Value, into map[uintptr]struct{}) {
	switch v.Kind() {
	case reflect.Slice:
		if v.Len() > 0 {
			into[v.Pointer()] = struct{}{}
		}
		for i := 0; i < v.Len(); i++ {
			sliceBackingArrays(v.Index(i), into)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			sliceBackingArrays(v.Field(i), into)
		}
	default:
	}
}

// This guards the whole spec tree rather than the fields clone happens to name
// today: a slice field added to any spec without a matching clone update shows
// up here as shared storage.
func TestNormalizeDesiredStateSharesNoSliceStorageWithCaller(t *testing.T) {
	t.Parallel()

	original := unnormalizedConfig()
	normalized := NormalizeDesiredState(original)

	before := map[uintptr]struct{}{}
	sliceBackingArrays(reflect.ValueOf(original), before)
	if len(before) == 0 {
		t.Fatal("fixture declares no populated slices, so this test would prove nothing")
	}

	after := map[uintptr]struct{}{}
	sliceBackingArrays(reflect.ValueOf(normalized), after)

	for addr := range after {
		if _, shared := before[addr]; shared {
			t.Fatalf("normalized config shares slice storage at %#x with the caller's config", addr)
		}
	}
}

func TestNormalizeDesiredStatePreservesNilVersusEmptySlices(t *testing.T) {
	t.Parallel()

	// normalize materializes the top-level collections, so a nil input must
	// still come back as the empty slice the encoder and validator expect.
	got := NormalizeDesiredState(OrganizationConfig{Organization: "acme"})
	if got.Members == nil || got.Invites == nil || got.Repositories == nil || got.Teams == nil {
		t.Fatalf("top-level collections must be materialized: %#v", got)
	}
}
