package folderfort

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/rest"
)

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the hash of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.hash, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData(context.TODO())
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	// FolderFort doesn't support setting modification time
	return fs.ErrorCantSetModTime
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// readMetaData gets the metadata if it hasn't already been fetched
func (o *Object) readMetaData(ctx context.Context) error {
	if o.hasMetaData {
		return nil
	}

	// We need to find this object by listing its parent directory
	dir, fileName := path.Split(o.remote)
	dir = path.Clean(dir)
	if dir == "." {
		dir = ""
	}

	// Encode the fileName for comparison: pad first, then encode
	encodedFileName := o.fs.opt.Enc.FromStandardName(o.fs.ensureMinLength(fileName))

	// Resolve directory relative to filesystem root
	targetDir := dir
	if dir == "" && o.fs.root != "" {
		targetDir = o.fs.root
	} else if dir != "" && o.fs.root != "" {
		targetDir = path.Join(o.fs.root, dir)
	}

	parentID, err := o.fs.getParentID(ctx, targetDir)
	if err != nil {
		// If the parent directory doesn't exist, the object doesn't exist either
		if err == fs.ErrorDirNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}

	entries, err := o.fs.listAll(ctx, parentID)
	if err != nil {
		// If we can't list the parent directory, the object doesn't exist
		return fs.ErrorObjectNotFound
	}

	for _, entry := range entries {
		if entry.Name == encodedFileName && entry.Type != "folder" {
			o.hasMetaData = true
			o.size = entry.FileSize
			o.modTime = entry.UpdatedAt
			o.id = entry.ID
			o.parentID = entry.ParentID
			o.mimeType = entry.Mime
			o.hash = entry.Hash
			o.downloadURL = entry.URL
			return nil
		}
	}

	return fs.ErrorObjectNotFound
} // Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if err := o.readMetaData(ctx); err != nil {
		return nil, err
	}

	// Debug: log the download URL
	fs.Debugf(o, "Using download URL: %s", o.downloadURL)

	// The download URL is relative to the base site URL, not the API URL
	// We need to create a new rest client with the base site URL
	baseURL := strings.TrimSuffix(o.fs.opt.URL, "/")
	client := fshttp.NewClient(context.Background())
	downloadClient := rest.NewClient(client).SetRoot(baseURL)
	downloadClient.SetHeader("Authorization", "Bearer "+o.fs.opt.AccessToken)

	opts := rest.Opts{
		Method: "GET",
		Path:   "/" + o.downloadURL, // Add leading slash since URL is relative
	}

	// Handle range requests
	headers := make(map[string]string)
	var start, end int64 = 0, -1

	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			start = x.Offset
		case *fs.RangeOption:
			if x.Start >= 0 {
				// Normal range: Start=100, End=-1 means "from offset 100 to end"
				// Normal range: Start=100, End=200 means "from offset 100 to 200"
				start = x.Start
				if x.End >= 0 {
					end = x.End
				}
			} else if x.Start == -1 && x.End >= 0 {
				// Suffix range: Start=-1, End=100 means "last 100 bytes"
				// Calculate the start offset: fileSize - x.End
				start = o.size - x.End
				if start < 0 {
					start = 0
				}
				end = o.size - 1
			}
		}
	}

	if start > 0 || end >= 0 {
		if end < 0 {
			end = o.size - 1
		}
		headers["Range"] = fmt.Sprintf("bytes=%d-%d", start, end)
		fs.Debugf(o, "Setting Range header: bytes=%d-%d for file size %d", start, end, o.size)
	}

	if len(headers) > 0 {
		opts.ExtraHeaders = headers
	}

	var resp *http.Response
	var err error
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = downloadClient.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return resp.Body, nil
}

// Update the object with the contents of the io.Reader, modTime and size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// FolderFort doesn't seem to have a direct update API, so we need to delete and recreate
	err := o.Remove(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove old object: %w", err)
	}

	// Create a wrapper ObjectInfo that uses this object's remote path
	// but the source's other properties (size, modtime, etc.)
	updateSrc := &updateObjectInfo{
		remote:  o.remote,  // Use the current object's path
		src:     src,       // Delegate other calls to the source
	}

	// Create new object at the same path
	newObj, err := o.fs.PutUnchecked(ctx, in, updateSrc, options...)
	if err != nil {
		return err
	}

	// Update this object with the new metadata
	newO := newObj.(*Object)
	o.hasMetaData = newO.hasMetaData
	o.size = newO.size
	o.modTime = newO.modTime
	o.id = newO.id
	o.parentID = newO.parentID
	o.mimeType = newO.mimeType
	o.hash = newO.hash
	o.downloadURL = newO.downloadURL

	return nil
}

// updateObjectInfo wraps an ObjectInfo to override the Remote() method
type updateObjectInfo struct {
	remote string
	src    fs.ObjectInfo
}

func (u *updateObjectInfo) Remote() string              { return u.remote }
func (u *updateObjectInfo) ModTime(ctx context.Context) time.Time { return u.src.ModTime(ctx) }
func (u *updateObjectInfo) Size() int64                { return u.src.Size() }
func (u *updateObjectInfo) Fs() fs.Info                { return u.src.Fs() }
func (u *updateObjectInfo) Hash(ctx context.Context, t hash.Type) (string, error) { return u.src.Hash(ctx, t) }
func (u *updateObjectInfo) Storable() bool             { return u.src.Storable() }
func (u *updateObjectInfo) String() string             { return u.remote }

// Remove this object
func (o *Object) Remove(ctx context.Context) error {
	if err := o.readMetaData(ctx); err != nil {
		return err
	}

	return o.fs.deleteEntry(ctx, []string{strconv.Itoa(o.id)}, false)
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return ""
	}
	return o.mimeType
}
