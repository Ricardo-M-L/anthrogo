package tool

import "github.com/ricardo/anthrogo/pkg/message"

// ResultType discriminates the payload variant.
type ResultType string

const (
	ResultText       ResultType = "text"
	ResultImage      ResultType = "image"
	ResultStructured ResultType = "structured"
)

// Result is what a Tool.Call returns. The TUI consumes ForUser; the engine
// pushes ForLLM (or a fallback to Text) back to the model as a tool_result block.
type Result struct {
	Type    ResultType
	Text    string               // canonical payload
	Image   *message.ImageSource // for ResultImage
	ForLLM  string               // what the model sees in tool_result
	ForUser string               // optional user-facing summary
	Data    map[string]any       // structured fields the TUI can read
	IsError bool
}

// ModelText returns the string to send back to the model. Falls back to Text
// when ForLLM is empty.
func (r Result) ModelText() string {
	if r.ForLLM != "" {
		return r.ForLLM
	}
	return r.Text
}
