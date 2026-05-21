package compact

import (
	"strings"
	"testing"

	"github.com/ricardo/anthrogo/pkg/message"
)

func BenchmarkApproxBytes_100Messages(b *testing.B) {
	msgs := make([]message.Message, 100)
	for i := range msgs {
		msgs[i] = message.Message{
			Role: message.RoleUser,
			Content: []message.Block{{
				Type: message.BlockText,
				Text: strings.Repeat("x", 200),
			}},
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ApproxBytes(msgs)
	}
}
