package generator

import (
	"testing"
	"time"
)

func BenchmarkGenerate5x5(b *testing.B) {
	for b.Loop() {
		_, err := Generate(5, 5*time.Second)
		if err != nil {
			b.Fatalf("Generate failed: %v", err)
		}
	}
}
