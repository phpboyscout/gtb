package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testData struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func TestWriter_JSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := NewWriter(&buf, FormatJSON)

	data := testData{Name: "test-tool", Version: "1.0.0"}

	err := w.Write(data, func(w io.Writer) {
		t.Error("text function should not be called for JSON output")
	})

	require.NoError(t, err)

	var result testData
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "test-tool", result.Name)
	assert.Equal(t, "1.0.0", result.Version)
}

func TestWriter_JSONIndent(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := NewWriter(&buf, FormatJSON)

	data := testData{Name: "tool", Version: "2.0.0"}
	err := w.Write(data, func(w io.Writer) {})

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "  \"name\"")
	assert.Contains(t, buf.String(), "  \"version\"")
}

func TestWriter_Text(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := NewWriter(&buf, FormatText)

	data := testData{Name: "test-tool", Version: "1.0.0"}
	textCalled := false

	err := w.Write(data, func(w io.Writer) {
		textCalled = true
		fmt.Fprintf(w, "%s %s\n", data.Name, data.Version)
	})

	require.NoError(t, err)
	assert.True(t, textCalled)
	assert.Equal(t, "test-tool 1.0.0\n", buf.String())
}

func TestWriter_DefaultIsText(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := NewWriter(&buf, Format("unknown"))

	textCalled := false

	err := w.Write(nil, func(w io.Writer) {
		textCalled = true
		fmt.Fprint(w, "hello")
	})

	require.NoError(t, err)
	assert.True(t, textCalled)
	assert.Equal(t, "hello", buf.String())
}

func TestWriter_IsJSON(t *testing.T) {
	t.Parallel()

	assert.True(t, NewWriter(nil, FormatJSON).IsJSON())
	assert.False(t, NewWriter(nil, FormatText).IsJSON())
	assert.False(t, NewWriter(nil, Format("yaml")).IsJSON())
}
