package config

import "testing"

func TestRepositorySpecSetManagedOptionals(t *testing.T) {
	t.Parallel()

	var repo RepositorySpec
	repo.SetManagedDescription("")
	repo.SetManagedHomepage("")
	repo.SetManagedAllowForking(false)
	repo.SetManagedArchived(false)
	repo.SetManagedIsTemplate(false)

	if got := repo.DescriptionOption(); got != optionalString("") {
		t.Fatalf("unexpected description option: %#v", got)
	}
	if got := repo.HomepageOption(); got != optionalString("") {
		t.Fatalf("unexpected homepage option: %#v", got)
	}
	if got := repo.AllowForkingOption(); got != optionalBool(false) {
		t.Fatalf("unexpected allow_forking option: %#v", got)
	}
	if got := repo.ArchivedOption(); got != optionalBool(false) {
		t.Fatalf("unexpected archived option: %#v", got)
	}
	if got := repo.IsTemplateOption(); got != optionalBool(false) {
		t.Fatalf("unexpected is_template option: %#v", got)
	}

	if value, managed := repo.ManagedDescription(); !managed || value != "" {
		t.Fatalf("expected managed empty description, got value=%q managed=%v", value, managed)
	}
	if value, managed := repo.ManagedHomepage(); !managed || value != "" {
		t.Fatalf("expected managed empty homepage, got value=%q managed=%v", value, managed)
	}
	if value, managed := repo.ManagedAllowForking(); !managed || value {
		t.Fatalf("expected managed false allow_forking, got value=%v managed=%v", value, managed)
	}
	if value, managed := repo.ManagedArchived(); !managed || value {
		t.Fatalf("expected managed false archived, got value=%v managed=%v", value, managed)
	}
	if value, managed := repo.ManagedIsTemplate(); !managed || value {
		t.Fatalf("expected managed false is_template, got value=%v managed=%v", value, managed)
	}
}

func TestRepositorySpecManagedOptionFallbackAndNullBehavior(t *testing.T) {
	t.Parallel()

	repo := RepositorySpec{
		Description:  "GitOps CLI",
		Homepage:     "https://example.com/repo-builder",
		AllowForking: true,
		Archived:     true,
		IsTemplate:   true,
	}

	if value, managed := repo.ManagedDescription(); !managed || value != "GitOps CLI" {
		t.Fatalf("expected fallback managed description, got value=%q managed=%v", value, managed)
	}
	if value, managed := repo.ManagedHomepage(); !managed || value != "https://example.com/repo-builder" {
		t.Fatalf("expected fallback managed homepage, got value=%q managed=%v", value, managed)
	}
	if value, managed := repo.ManagedAllowForking(); !managed || !value {
		t.Fatalf("expected fallback managed allow_forking, got value=%v managed=%v", value, managed)
	}
	if value, managed := repo.ManagedArchived(); !managed || !value {
		t.Fatalf("expected fallback managed archived, got value=%v managed=%v", value, managed)
	}
	if value, managed := repo.ManagedIsTemplate(); !managed || !value {
		t.Fatalf("expected fallback managed is_template, got value=%v managed=%v", value, managed)
	}

	repo.description = nullOptionalString()
	repo.homepage = nullOptionalString()
	repo.allowForking = nullOptionalBool()
	repo.archived = nullOptionalBool()
	repo.isTemplate = nullOptionalBool()

	if _, managed := repo.ManagedDescription(); managed {
		t.Fatalf("expected explicit null description to remain unmanaged")
	}
	if _, managed := repo.ManagedHomepage(); managed {
		t.Fatalf("expected explicit null homepage to remain unmanaged")
	}
	if _, managed := repo.ManagedAllowForking(); managed {
		t.Fatalf("expected explicit null allow_forking to remain unmanaged")
	}
	if _, managed := repo.ManagedArchived(); managed {
		t.Fatalf("expected explicit null archived to remain unmanaged")
	}
	if _, managed := repo.ManagedIsTemplate(); managed {
		t.Fatalf("expected explicit null is_template to remain unmanaged")
	}
}
