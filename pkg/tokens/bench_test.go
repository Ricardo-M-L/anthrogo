package tokens

import (
	"strings"
	"testing"
)

func BenchmarkCountText_OpenAI(b *testing.B) {
	c := NewCounter("gpt-4o")
	text := strings.Repeat("hello world ", 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.CountText(text)
	}
}

func BenchmarkCountText_Anthropic(b *testing.B) {
	c := NewCounter("claude-sonnet-4-6")
	text := strings.Repeat("hello world ", 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.CountText(text)
	}
}
