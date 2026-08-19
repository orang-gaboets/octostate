package config

import (
	"reflect"
	"testing"
)

func unnormalizedConfig() OrganizationConfig {
	repo := RepositorySpec{
		Name: " service ", Visibility: " private ", Topics: []string{" go "},
		Template: TemplateSpec{Owner: " acme ", Name: " base "},
	}
	// The managed setters are the programmatic equivalent of declaring these
	// fields in YAML, so normalization must reach the presence-aware copy too.
	repo.SetManagedDescription(" a service ")
	repo.SetManagedHomepage(" https://example.com/service ")

	return OrganizationConfig{
		Organization: " acme ",
		Members: []OrganizationMemberSpec{
			{Username: " alice ", Role: " member "},
		},
		Invites: []InviteSpec{
			{Username: optionalString(" octocat "), Role: " direct_member ", TeamSlugs: []string{" platform "}},
			{Email: optionalString(" dev@example.com "), Role: " direct_member ", TeamSlugs: []string{" platform "}},
		},
		Repositories: []RepositorySpec{repo},
		Teams: []TeamSpec{
			{Slug: " parent ", Name: " Parent ", Description: " parent desc ", Privacy: " closed "},
			{Slug: " platform ", Name: " Platform ", Description: " desc ", Privacy: " closed ", ParentSlug: " parent ",
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
	if got.Invites[1].Email.Value != "dev@example.com" {
		t.Fatalf("invite email: %#v", got.Invites[1])
	}
	repo := got.Repositories[0]
	if repo.Owner != "acme" || repo.Name != "service" || repo.Visibility != "private" || repo.Topics[0] != "go" {
		t.Fatalf("repository: %#v", repo)
	}
	if repo.Template.Owner != "acme" || repo.Template.Name != "base" {
		t.Fatalf("template: %#v", repo.Template)
	}
	if repo.Description != "a service" || repo.Homepage != "https://example.com/service" {
		t.Fatalf("repository description/homepage: %#v", repo)
	}
	// Normalization must trim the presence-aware copy in step with the plain
	// field, or the managed value reconciled upstream stays padded.
	if got, managed := repo.ManagedDescription(); !managed || got != "a service" {
		t.Fatalf("managed description = %q managed=%v", got, managed)
	}
	if got, managed := repo.ManagedHomepage(); !managed || got != "https://example.com/service" {
		t.Fatalf("managed homepage = %q managed=%v", got, managed)
	}
	team := got.Teams[1]
	if team.Slug != "platform" || team.Name != "Platform" || team.Description != "desc" || team.Privacy != "closed" {
		t.Fatalf("team: %#v", team)
	}
	if team.ParentSlug != "parent" {
		t.Fatalf("team parent slug: %q", team.ParentSlug)
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

// sliceFieldPaths walks a type tree and names every slice field reachable from
// it, so fixture coverage can be checked against the types themselves rather
// than against the fields someone remembered to populate.
func sliceFieldPaths(t reflect.Type, prefix string, into map[string]struct{}) {
	switch t.Kind() {
	case reflect.Slice:
		into[prefix] = struct{}{}
		sliceFieldPaths(t.Elem(), prefix+"[]", into)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			sliceFieldPaths(field.Type, prefix+"."+field.Name, into)
		}
	default:
	}
}

// populatedSlicePaths names the slice fields the fixture actually fills.
func populatedSlicePaths(v reflect.Value, prefix string, into map[string]struct{}) {
	switch v.Kind() {
	case reflect.Slice:
		if v.Len() == 0 {
			return
		}
		into[prefix] = struct{}{}
		for i := 0; i < v.Len(); i++ {
			populatedSlicePaths(v.Index(i), prefix+"[]", into)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			populatedSlicePaths(v.Field(i), prefix+"."+v.Type().Field(i).Name, into)
		}
	default:
	}
}

// The shared-storage test below can only observe a slice the fixture populates,
// so this keeps the fixture honest: a slice field added to any spec fails here
// until unnormalizedConfig fills it, which is what makes the shared-storage
// guard extend to fields that do not exist yet.
func TestUnnormalizedConfigPopulatesEverySliceField(t *testing.T) {
	t.Parallel()

	declared := map[string]struct{}{}
	sliceFieldPaths(reflect.TypeOf(OrganizationConfig{}), "", declared)

	populated := map[string]struct{}{}
	populatedSlicePaths(reflect.ValueOf(unnormalizedConfig()), "", populated)

	for path := range declared {
		if _, ok := populated[path]; !ok {
			t.Errorf("unnormalizedConfig leaves %s empty, so the shared-storage guard cannot see it", path)
		}
	}
}

// This guards the whole spec tree rather than the fields clone happens to name
// today: a slice field added to any spec shows up here as shared storage, since
// TestUnnormalizedConfigPopulatesEverySliceField forces the fixture to fill it.
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

func TestNormalizeDesiredStateMaterializesNilCollections(t *testing.T) {
	t.Parallel()

	// normalize materializes the top-level collections, so a nil input must
	// still come back as the empty slice the encoder and validator expect.
	got := NormalizeDesiredState(OrganizationConfig{Organization: "acme"})
	if got.Members == nil || got.Invites == nil || got.Repositories == nil || got.Teams == nil {
		t.Fatalf("top-level collections must be materialized: %#v", got)
	}
}
