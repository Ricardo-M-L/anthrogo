package message

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// pngOnePixel is a minimal 1x1 PNG byte literal.
var pngOnePixel = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
	0x54, 0x08, 0x99, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
	0x00, 0x00, 0x03, 0x00, 0x01, 0x6D, 0x09, 0xB5,
	0x68, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
	0x44, 0xAE, 0x42, 0x60, 0x82,
}

func writeTempPNG(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "img.png")
	require.NoError(t, os.WriteFile(p, pngOnePixel, 0600))
	return p
}

func TestParseUserPrompt_PlainText(t *testing.T) {
	blocks, err := ParseUserPrompt("hello world")
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, BlockText, blocks[0].Type)
	require.Equal(t, "hello world", blocks[0].Text)
}

func TestParseUserPrompt_EmptyString(t *testing.T) {
	blocks, err := ParseUserPrompt("")
	require.NoError(t, err)
	require.Nil(t, blocks)
}

func TestParseUserPrompt_SingleImage(t *testing.T) {
	p := writeTempPNG(t)
	blocks, err := ParseUserPrompt("@image:" + p)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, BlockImage, blocks[0].Type)
	require.NotNil(t, blocks[0].ImageSource)
	require.Equal(t, "image/png", blocks[0].ImageSource.MediaType)
	require.Equal(t, "base64", blocks[0].ImageSource.Type)
	require.NotEmpty(t, blocks[0].ImageSource.Data)
}

func TestParseUserPrompt_TextBeforeAndAfterImage(t *testing.T) {
	p := writeTempPNG(t)
	prompt := "before @image:" + p + " after"
	blocks, err := ParseUserPrompt(prompt)
	require.NoError(t, err)
	require.Len(t, blocks, 3)
	require.Equal(t, BlockText, blocks[0].Type)
	require.Equal(t, "before ", blocks[0].Text)
	require.Equal(t, BlockImage, blocks[1].Type)
	require.Equal(t, BlockText, blocks[2].Type)
	require.Equal(t, " after", blocks[2].Text)
}

func TestParseUserPrompt_MultipleImages(t *testing.T) {
	p1 := writeTempPNG(t)
	p2 := writeTempPNG(t)
	prompt := "@image:" + p1 + " text @image:" + p2
	blocks, err := ParseUserPrompt(prompt)
	require.NoError(t, err)
	// [image, text, image]
	require.Len(t, blocks, 3)
	require.Equal(t, BlockImage, blocks[0].Type)
	require.Equal(t, BlockText, blocks[1].Type)
	require.Equal(t, BlockImage, blocks[2].Type)
}

func TestParseUserPrompt_UnsupportedMime(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(p, []byte("hello text file"), 0600))
	_, err := ParseUserPrompt("@image:" + p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported image MIME")
}

func TestParseUserPrompt_MissingFile(t *testing.T) {
	_, err := ParseUserPrompt("@image:/nonexistent/path/img.png")
	require.Error(t, err)
}
