package diff

import (
	"testing"
)

func BenchmarkBuildActionsConcurrency(b *testing.B) {
	builder := largeFixtureBuilder()

	benchmark := func(b *testing.B, limit int) {
		b.Helper()
		b.ReportAllocs()
		for b.Loop() {
			if _, err := builder.buildActionsWithLimit(limit); err != nil {
				b.Fatalf("buildActionsWithLimit returned error: %v", err)
			}
		}
	}

	b.Run("sequential", func(b *testing.B) {
		benchmark(b, 1)
	})

	b.Run("bounded_concurrent", func(b *testing.B) {
		benchmark(b, diffPhaseConcurrency)
	})
}
