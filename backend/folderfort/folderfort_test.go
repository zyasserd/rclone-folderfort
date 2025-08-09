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

		// TODO: make sure that they are all actually unimplementable
		// Skip unimplemented Fs methods
		UnimplementableFsMethods: []string{
			"Command",         // No custom commands
			"OpenWriterAt",    // No WriteAt support
			"OpenChunkWriter", // No chunk writer
			"ChangeNotify",    // No change notifications
			"PutStream",       // No stream upload (regular upload only)
			"CleanUp",         // No cleanup functionality
			"UserInfo",        // No user info
			"Disconnect",      // No disconnect needed
			"MergeDirs",       // No merge directories
			"DirSetModTime",   // No directory modtime support
		},
		// Skip unimplemented Object methods
		UnimplementableObjectMethods: []string{
			"SetModTime", // No modtime setting support
			"GetTier",    // No storage tiers
			"SetTier",    // No storage tiers
			"ID",         // No unique object IDs exposed
		},
	})
}
