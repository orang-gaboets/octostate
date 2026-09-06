package syncfromlive

import (
	"testing"

	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestBootstrapOwner(t *testing.T) {
	t.Parallel()

	if got := bootstrapOwner("orang-gaboets", " orang-gaboets "); got != "" {
		t.Fatalf("expected org owner to be omitted, got %q", got)
	}
	if got := bootstrapOwner("orang-gaboets", "Shared-Platform"); got != "Shared-Platform" {
		t.Fatalf("expected external owner to be preserved, got %q", got)
	}
	if got := bootstrapOwner("orang-gaboets", "   "); got != "" {
		t.Fatalf("expected blank owner to stay omitted, got %q", got)
	}
}

func TestBootstrapRepositoriesMaterializesManagedFields(t *testing.T) {
	t.Parallel()

	got := bootstrapRepositories("orang-gaboets", []state.Repository{
		{
			Owner:        " orang-gaboets ",
			Name:         " octostate ",
			Visibility:   " private ",
			Description:  "GitOps CLI",
			Homepage:     "https://example.com/octostate",
			Topics:       []string{"gitops", "go"},
			AllowForking: true,
			Archived:     true,
			IsTemplate:   false,
		},
		{
			Owner:        "shared-platform",
			Name:         "shared-repo",
			Visibility:   "public",
			Description:  "",
			Homepage:     "",
			Topics:       []string{"alpha"},
			AllowForking: false,
			Archived:     false,
			IsTemplate:   true,
		},
	})

	if len(got) != 2 {
		t.Fatalf("expected two repositories, got %#v", got)
	}

	privateRepo := got[0]
	if privateRepo.Owner != "" || privateRepo.Name != "octostate" || privateRepo.Visibility != "private" {
		t.Fatalf("unexpected private repo bootstrap result: %#v", privateRepo)
	}
	if value, managed := privateRepo.ManagedAllowForking(); !managed || !value {
		t.Fatalf("expected managed private allow_forking=true, got value=%v managed=%v", value, managed)
	}
	if value, managed := privateRepo.ManagedDescription(); !managed || value != "GitOps CLI" {
		t.Fatalf("expected managed description, got value=%q managed=%v", value, managed)
	}
	if value, managed := privateRepo.ManagedArchived(); !managed || !value {
		t.Fatalf("expected managed archived=true, got value=%v managed=%v", value, managed)
	}

	publicRepo := got[1]
	if publicRepo.Owner != "shared-platform" {
		t.Fatalf("expected external owner, got %#v", publicRepo.Owner)
	}
	if _, managed := publicRepo.ManagedAllowForking(); managed {
		t.Fatalf("expected public allow_forking to stay unmanaged, got %#v", publicRepo.AllowForkingOption())
	}
	if value, managed := publicRepo.ManagedHomepage(); !managed || value != "" {
		t.Fatalf("expected explicit empty managed homepage, got value=%q managed=%v", value, managed)
	}
	if value, managed := publicRepo.ManagedIsTemplate(); !managed || !value {
		t.Fatalf("expected managed is_template=true, got value=%v managed=%v", value, managed)
	}
}

func TestBootstrapRepositoriesPreservesInternalVisibility(t *testing.T) {
	t.Parallel()

	got := bootstrapRepositories("acme", []state.Repository{{
		Owner: "acme", Name: "platform", Visibility: "internal", AllowForking: true,
	}})
	if len(got) != 1 || got[0].Visibility != "internal" {
		t.Fatalf("expected internal visibility to be preserved, got %#v", got)
	}
	if value, managed := got[0].ManagedAllowForking(); !managed || !value {
		t.Fatalf("expected internal allow_forking=true to be managed, got value=%v managed=%v", value, managed)
	}
}
