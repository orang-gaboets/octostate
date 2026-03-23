package syncfromlive

import (
	"testing"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
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
			Name:         " repo-builder ",
			Visibility:   " private ",
			Description:  "GitOps CLI",
			Homepage:     "https://example.com/repo-builder",
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
	if privateRepo.Owner != "" || privateRepo.Name != "repo-builder" || privateRepo.Visibility != "private" {
		t.Fatalf("unexpected private repo bootstrap result: %#v", privateRepo)
	}
	if _, managed := privateRepo.ManagedAllowForking(); managed {
		t.Fatalf("expected private allow_forking to stay unmanaged, got %#v", privateRepo.AllowForkingOption())
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
	if value, managed := publicRepo.ManagedAllowForking(); !managed || value {
		t.Fatalf("expected managed allow_forking=false, got value=%v managed=%v", value, managed)
	}
	if value, managed := publicRepo.ManagedHomepage(); !managed || value != "" {
		t.Fatalf("expected explicit empty managed homepage, got value=%q managed=%v", value, managed)
	}
	if value, managed := publicRepo.ManagedIsTemplate(); !managed || !value {
		t.Fatalf("expected managed is_template=true, got value=%v managed=%v", value, managed)
	}
}
