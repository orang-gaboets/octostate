// Package output provides functionalities and helpers for writing
// command results to stdout from the repo-builder CLI.
//
// This package centralizes command-layer output formatting so Cobra
// commands can return structured, user-visible results consistently
// without depending on service-layer logging behavior. It is intended
// for rendering command payloads such as JSON objects and arrays.
package output
