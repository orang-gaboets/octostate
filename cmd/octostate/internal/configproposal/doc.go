// Package configproposal applies validated local mutations to organization
// configuration files.
//
// A proposal loads the target file, validates it, applies one caller-supplied
// mutation, validates the result, and replaces the file atomically only when
// the canonical encoding actually changes. Callers use it to record a
// requested change in desired state instead of performing the equivalent live
// GitHub operation, so the mutation can be reviewed before it is applied.
package configproposal
