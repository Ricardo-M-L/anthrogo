package message

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
)

var imageRefRe = regexp.MustCompile(`@image:([^\s]+)`)

// ParseUserPrompt converts a user prompt string into a slice of Blocks.
// Recognizes "@image:<path>" tokens anywhere in the string; each matched
// token loads the file, base64-encodes it, and emits a BlockImage at the
// same position. Text on either side of image refs is preserved as
// BlockText blocks. Returns an error if any referenced file cannot be read.
func ParseUserPrompt(prompt string) ([]Block, error) {
	matches := imageRefRe.FindAllStringSubmatchIndex(prompt, -1)
	if len(matches) == 0 {
		if strings.TrimSpace(prompt) == "" {
			return nil, nil
		}
		return []Block{{Type: BlockText, Text: prompt}}, nil
	}
	var blocks []Block
	cursor := 0
	for _, m := range matches {
		// m: [start, end, group1Start, group1End]
		before := prompt[cursor:m[0]]
		if strings.TrimSpace(before) != "" {
			blocks = append(blocks, Block{Type: BlockText, Text: before})
		}
		path := prompt[m[2]:m[3]]
		src, err := loadImageSource(path)
		if err != nil {
			return nil, fmt.Errorf("@image:%s: %w", path, err)
		}
		blocks = append(blocks, Block{Type: BlockImage, ImageSource: src})
		cursor = m[1]
	}
	if cursor < len(prompt) {
		rest := prompt[cursor:]
		if strings.TrimSpace(rest) != "" {
			blocks = append(blocks, Block{Type: BlockText, Text: rest})
		}
	}
	return blocks, nil
}

func loadImageSource(path string) (*ImageSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	mime := http.DetectContentType(data)
	// Normalize to one of the four MIME types Anthropic + OpenAI both accept.
	switch {
	case strings.HasPrefix(mime, "image/png"):
		mime = "image/png"
	case strings.HasPrefix(mime, "image/jpeg"), strings.HasPrefix(mime, "image/jpg"):
		mime = "image/jpeg"
	case strings.HasPrefix(mime, "image/gif"):
		mime = "image/gif"
	case strings.HasPrefix(mime, "image/webp"):
		mime = "image/webp"
	default:
		return nil, fmt.Errorf("unsupported image MIME: %s", mime)
	}
	return &ImageSource{
		Type:      "base64",
		MediaType: mime,
		Data:      base64.StdEncoding.EncodeToString(data),
	}, nil
}
