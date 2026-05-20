package compact

import (
	"encoding/json"

	"github.com/ricardo/anthrogo/pkg/message"
)

// ApproxBytes is a stand-in for token counting. Returns the sum of JSON-
// marshaled byte lengths across all messages.
//
// Deprecated: Use pkg/tokens.Counter.CountMessages for accurate token counts.
// ApproxBytes is retained for tests and legacy callers but is no longer called
// by compact.Run (replaced in M8.10).
func ApproxBytes(msgs []message.Message) int {
	total := 0
	for i := range msgs {
		raw, err := json.Marshal(msgs[i])
		if err != nil {
			continue
		}
		total += len(raw)
	}
	return total
}
