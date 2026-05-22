package permissions

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkDecide_NoRules(b *testing.B) {
	c := Empty()
	input := map[string]any{"command": "ls"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decide(context.Background(), c, "Bash", input)
	}
}

func BenchmarkDecide_ManyAlwaysAllow(b *testing.B) {
	c := Empty()
	var rules []Rule
	for i := 0; i < 100; i++ {
		rules = append(rules, Rule{Tool: "Bash", Pattern: fmt.Sprintf("cmd-%d*", i)})
	}
	c.AlwaysAllowRules[SourceUser] = rules
	input := map[string]any{"command": "cmd-50 args"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decide(context.Background(), c, "Bash", input)
	}
}
