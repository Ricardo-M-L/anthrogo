package compact

import (
	"encoding/json"

	"github.com/ricardo/anthrogo/pkg/message"
)

// ApproxBytes is a stand-in for token counting. Returns the sum of JSON-
// marshaled byte lengths across all messages. Good enough for M4.2; a real
// tokenizer arrives with M6 (multi-provider).
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
