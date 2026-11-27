package grabber

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type remoteFileInfo struct {
	size    int64
	modTime time.Time
}

func (f remoteFileInfo) Name() string       { return "file.zip" }
func (f remoteFileInfo) Size() int64        { return f.size }
func (f remoteFileInfo) Mode() os.FileMode  { return 0 }
func (f remoteFileInfo) ModTime() time.Time { return f.modTime }
func (f remoteFileInfo) IsDir() bool        { return false }
func (f remoteFileInfo) Sys() interface{}   { return nil }

func TestLocalFileMatchesRemote(t *testing.T) {
	tempfile, err := os.CreateTemp(t.TempDir(), "ftpgrab-test")
	require.NoError(t, err)
	require.NoError(t, tempfile.Close())

	modTime := time.Date(2026, time.November, 1, 5, 0, 21, 0, time.UTC)
	require.NoError(t, os.Chtimes(tempfile.Name(), modTime, modTime))
	local, err := os.Stat(tempfile.Name())
	require.NoError(t, err)

	require.True(t, localFileMatchesRemote(local, remoteFileInfo{size: -1, modTime: modTime}))
	require.False(t, localFileMatchesRemote(local, remoteFileInfo{size: -1, modTime: modTime.Add(time.Second)}))
	require.True(t, localFileMatchesRemote(local, remoteFileInfo{size: local.Size(), modTime: modTime.Add(time.Second)}))
	require.False(t, localFileMatchesRemote(local, remoteFileInfo{size: local.Size() + 1, modTime: modTime}))
}
