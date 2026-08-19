package config

// NormalizeDesiredState returns the canonical form of cfg without mutating the
// caller's value.
//
// Loading configuration from disk normalizes it in place, so every value that
// reaches reconciliation through LoadFile or LoadDir is already canonical.
// Configuration constructed programmatically - by tests, by library consumers,
// or by commands that assemble desired state in memory - is not. Reconciliation
// compares desired state against live GitHub state field by field, so an
// un-normalized value such as " private " reads as drift against the live
// "private" and an un-normalized team slug reads as a different team entirely.
//
// Reconciliation entry points call this so both paths agree. It applies exactly
// the normalization LoadFile applies and nothing more: normalization makes
// equivalent declarations compare equal, it never repairs an invalid one.
// Validate remains the boundary that decides semantic validity.
func NormalizeDesiredState(cfg OrganizationConfig) OrganizationConfig {
	normalized := cfg.clone()
	normalize(&normalized)
	return normalized
}

// clone returns a deep copy that shares no slice backing array with cfg, so
// normalizing the copy cannot write through into the caller's value.
func (cfg OrganizationConfig) clone() OrganizationConfig {
	cfg.Members = cloneSlice(cfg.Members)
	cfg.Invites = cloneSlice(cfg.Invites)
	for i := range cfg.Invites {
		cfg.Invites[i].TeamSlugs = cloneSlice(cfg.Invites[i].TeamSlugs)
	}
	cfg.Repositories = cloneSlice(cfg.Repositories)
	for i := range cfg.Repositories {
		cfg.Repositories[i].Topics = cloneSlice(cfg.Repositories[i].Topics)
	}
	cfg.Teams = cloneSlice(cfg.Teams)
	for i := range cfg.Teams {
		cfg.Teams[i].Members = cloneSlice(cfg.Teams[i].Members)
		cfg.Teams[i].Repositories = cloneSlice(cfg.Teams[i].Repositories)
	}
	return cfg
}

// cloneSlice copies src, leaving a nil slice nil so the copy is an exact
// starting point for normalize. normalize decides what a nil collection means:
// it materializes the top-level ones as empty slices and leaves the rest alone.
func cloneSlice[T any](src []T) []T {
	if src == nil {
		return nil
	}
	return append(make([]T, 0, len(src)), src...)
}
