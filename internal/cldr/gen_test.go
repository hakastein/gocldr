package cldr_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hakastein/gocldr/internal/cldr"
)

func TestSortedKeys(t *testing.T) {
	assert.Empty(t, cldr.SortedKeys(map[string]int{}))
	assert.Equal(t, []string{"a", "b", "c"},
		cldr.SortedKeys(map[string]int{"c": 3, "a": 1, "b": 2}))
}

func TestWriteStrMap(t *testing.T) {
	var buf bytes.Buffer
	cldr.WriteStrMap(&buf, "Empty", nil)
	assert.Empty(t, buf.String(), "empty map emits nothing")

	cldr.WriteStrMap(&buf, "M", map[string]string{"b": "2", "a": "1"})
	assert.Equal(t, `M: map[string]string{"a": "1", "b": "2", }, `, buf.String())
}

func TestWriteFormatted(t *testing.T) {
	dir := t.TempDir()

	t.Run("formats valid source", func(t *testing.T) {
		path := filepath.Join(dir, "ok.go")
		require.NoError(t, cldr.WriteFormatted(path, []byte("package x\nvar  A   =  1\n")))
		out, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "package x\n\nvar A = 1\n", string(out))
	})

	t.Run("dumps .broken on invalid source", func(t *testing.T) {
		path := filepath.Join(dir, "bad.go")
		err := cldr.WriteFormatted(path, []byte("not go at all"))
		require.Error(t, err)
		assert.NoFileExists(t, path)
		assert.FileExists(t, path+".broken")
	})
}
