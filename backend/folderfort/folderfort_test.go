// Test FolderFort filesystem interface
package folderfort_test

import (
	"testing"

	"github.com/rclone/rclone/backend/folderfort"
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
		NilObject:  (*folderfort.Object)(nil),
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize: 5 * fs.Mebi, // FolderFort S3 multipart has a minimum of 5MB
		},
	})
}
