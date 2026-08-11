// Package filereplace atomically replaces an existing file with new contents.
//
// Replacement writes to a temporary file in the destination directory and then
// swaps it into place, so a failure leaves the original file untouched rather
// than partially written. Platform-specific behavior is isolated so Windows can
// use its own replacement primitive and preserve recovery files when a
// replacement cannot be completed.
package filereplace
