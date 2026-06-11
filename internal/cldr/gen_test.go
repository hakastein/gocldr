package cldr_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hakastein/gocldr/internal/cldr"
)

func TestLoadJSON(t *testing.T) {
	t.Run("missing file reports false", func(t *testing.T) {
		var v map[string]string
		assert.False(t, cldr.LoadJSON(filepath.Join(t.TempDir(), "absent.json"), &v))
	})

	t.Run("existing file reports true", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ok.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"k": "v"}`), 0o644))
		var v map[string]string
		assert.True(t, cldr.LoadJSON(path, &v))
		assert.Equal(t, map[string]string{"k": "v"}, v)
	})

	t.Run("read error other than missing is fatal", func(t *testing.T) {
		if os.Getenv("GOCLDR_TEST_LOADJSON_FATAL") == "1" {
			var v map[string]string
			cldr.LoadJSON(t.TempDir(), &v) // a directory: read fails with a non-ErrNotExist error
			return
		}
		cmd := exec.Command(os.Args[0], "-test.run=TestLoadJSON/read_error")
		cmd.Env = append(os.Environ(), "GOCLDR_TEST_LOADJSON_FATAL=1")
		out, err := cmd.CombinedOutput()
		var exitErr *exec.ExitError
		require.ErrorAsf(t, err, &exitErr, "want non-zero exit, got err=%v output=%q", err, out)
		assert.Contains(t, string(out), "gen: read")
	})
}

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

func TestPruneLocaleDirs(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"en", "ru", "all", "stale"} {
		require.NoError(t, os.Mkdir(filepath.Join(dir, d), 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "loose.txt"), nil, 0o644))

	cldr.PruneLocaleDirs(dir, []string{"en", "ru"})

	assert.DirExists(t, filepath.Join(dir, "en"))
	assert.DirExists(t, filepath.Join(dir, "ru"))
	assert.DirExists(t, filepath.Join(dir, "all"), "the all aggregator is never pruned")
	assert.NoDirExists(t, filepath.Join(dir, "stale"))
	assert.FileExists(t, filepath.Join(dir, "loose.txt"), "non-directories are left alone")
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
