// Test FolderFort filesystem interface
package folderfort

import (
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as no remote name")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName: *fstest.RemoteName,
		NilObject:  (*Object)(nil),
	})
}

// TestNew tests creation of new filesystem
func TestNew(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as no remote name")
	}
	
	// Create a temporary config
	m := fstest.MakeRemoteName("folderfort", nil)
	f, err := fs.NewFs(m)
	if err != nil {
		t.Fatalf("Failed to create new Fs: %v", err)
	}
	if f == nil {
		t.Fatal("Expected non-nil Fs")
	}
}
