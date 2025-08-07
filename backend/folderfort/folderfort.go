// Package folderfort provides an interface to the FolderFort storage system.
package folderfort

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/folderfort/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "folderfort",
		Description: "FolderFort Cloud Storage",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "url",
			Help:     "URL of your FolderFort instance (e.g., https://yoursite.com)",
			Required: true,
		}, {
			Name:      "access_token",
			Help:      "Access token from FolderFort account settings or login",
			Required:  true,
			Sensitive: true,
		}, {
			Name:     "workspace_id",
			Help:     "Workspace ID (default: 0 for personal workspace)",
			Default:  0,
			Advanced: true,
		}, {
			Name:     "chunk_size",
			Help:     `Upload chunk size. Must be a power of 2 >= 256k.

Any files larger than this will be uploaded in chunks of this size.
The chunk size must be a power of 2 and at least 256k. Making it larger
will reduce the number of API calls needed to upload a file, but will use
more memory. The default is usually a good choice.
`,
			Default:  fs.SizeSuffix(50 * 1024 * 1024), // 50MB default chunk size  
			Advanced: true,
		}, {
			Name:     "upload_concurrency",
			Help:     `Concurrency for chunked uploads.

This is the number of chunks of the same file that are uploaded
concurrently for chunked uploads.

NB if you set this to > 1 then the checksums of chunks will be
incorrect. This speeds up transfers substantially though.

If you are uploading small numbers of large files over high speed links
and these uploads do not fully utilize your bandwidth, then increasing
this may help to speed up the transfers.`,
			Default:  1,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Base |
				encoder.EncodeInvalidUtf8 |
				encoder.EncodeSlash |
				encoder.EncodeCtl),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	URL               string               `config:"url"`
	AccessToken       string               `config:"access_token"`
	WorkspaceID       int                  `config:"workspace_id"`
	ChunkSize         fs.SizeSuffix        `config:"chunk_size"`
	UploadConcurrency int                  `config:"upload_concurrency"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote FolderFort server
type Fs struct {
	name       string         // name of this remote
	root       string         // the path we are working on
	opt        Options        // parsed options
	features   *fs.Features   // optional features
	srv        *rest.Client   // the connection to the server
	pacer      *fs.Pacer     // pacer for API calls
	precision  time.Duration  // precision of dates from the server
}

// Object describes a FolderFort file
type Object struct {
	fs          *Fs           // what this object is part of
	remote      string        // The remote path
	hasMetaData bool          // whether info below has been set
	size        int64         // size of the object
	modTime     time.Time     // modification time of the object
	id          int           // FolderFort ID of the object
	parentID    *int          // parent folder ID
	mimeType    string        // Content-Type of the object
	hash        string        // hash of the object
	downloadURL string        // download URL from the API
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	if opt.URL == "" {
		return nil, fmt.Errorf("FolderFort URL is required")
	}
	if opt.AccessToken == "" {
		return nil, fmt.Errorf("access token is required")
	}

	// Clean up the URL
	opt.URL = strings.TrimSuffix(opt.URL, "/")
	if !strings.HasPrefix(opt.URL, "http://") && !strings.HasPrefix(opt.URL, "https://") {
		opt.URL = "https://" + opt.URL
	}

	root = strings.Trim(root, "/")
	
	client := fshttp.NewClient(ctx)
	
	f := &Fs{
		name:      name,
		root:      root,
		opt:       *opt,
		srv:       rest.NewClient(client).SetRoot(opt.URL + "/api/v1"),
		pacer:     fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		precision: time.Second,
	}

	// Set authorization header
	f.srv.SetHeader("Authorization", "Bearer "+opt.AccessToken)
	f.srv.SetHeader("Accept", "application/json")

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		ReadMimeType:            true,
		WriteMimeType:           true,
	}).Fill(ctx, f)

	// Check if root is actually a file
	if root != "" {
		// Try to get the parent directory and look for the root item
		parentPath := path.Dir(root)
		if parentPath == "." {
			parentPath = ""
		}
		itemName := path.Base(root)
		
		parentID, err := f.getParentID(ctx, parentPath)
		if err == nil {
			entries, err := f.listAll(ctx, parentID)
			if err == nil {
				for _, entry := range entries {
					if entry.Name == itemName {
						if entry.Type != "folder" {
							// Root is a file, not a directory
							f.root = parentPath
							return f, fs.ErrorIsFile
						}
						// It's a directory, we're good
						break
					}
				}
			}
		}
	}

	return f, nil
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("FolderFort root '%s'", f.root)
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return f.precision
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	// FolderFort doesn't seem to provide file hashes in the API
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Try to decode error response
	if resp.Body != nil {
		defer resp.Body.Close()
		var errResp api.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			return fmt.Errorf("FolderFort error %d: %s", resp.StatusCode, errResp.Message)
		}
	}
	return fmt.Errorf("FolderFort error %d: %s", resp.StatusCode, resp.Status)
}

// shouldRetry returns true if we should retry this error
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if resp != nil && resp.StatusCode == 429 {
		return true, err
	}
	return fserrors.ShouldRetry(err), err
}

// getParentID gets the parent folder ID for a given path
func (f *Fs) getParentID(ctx context.Context, dirPath string) (*int, error) {
	if dirPath == "" || dirPath == "/" {
		return nil, nil // root directory
	}

	// Split the path into parts
	parts := strings.Split(strings.Trim(dirPath, "/"), "/")
	var parentID *int

	// Navigate through each part of the path
	for _, part := range parts {
		if part == "" {
			continue
		}

		// List entries in current directory to find the next part
		entries, err := f.listAll(ctx, parentID)
		if err != nil {
			return nil, err
		}

		found := false
		for _, entry := range entries {
			if entry.Name == part && entry.Type == "folder" {
				parentID = &entry.ID
				found = true
				break
			}
		}

		if !found {
			return nil, fs.ErrorDirNotFound
		}
	}

	return parentID, nil
}

// listAll lists all files and directories in the given parent directory
func (f *Fs) listAll(ctx context.Context, parentID *int) ([]api.FileEntry, error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/drive/file-entries",
		Parameters: url.Values{
			"perPage": {"1000"}, // Get a large number
		},
	}

	if parentID != nil {
		opts.Parameters.Set("parentIds", strconv.Itoa(*parentID))
		fs.Debugf(f, "Listing files with parentID: %d", *parentID)
	} else {
		fs.Debugf(f, "Listing files in root (no parentID)")
	}

	if f.opt.WorkspaceID != 0 {
		opts.Parameters.Set("workspaceId", strconv.Itoa(f.opt.WorkspaceID))
	}

	fs.Debugf(f, "API call parameters: %v", opts.Parameters)

	var entries []api.FileEntry
	var result interface{}
	
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to call API: %w", err)
	}

	// Convert result back to JSON and try to parse
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	// Try to parse as direct array first
	if err := json.Unmarshal(resultBytes, &entries); err == nil {
		fs.Debugf(f, "Parsed as direct array, found %d entries", len(entries))
		return entries, nil
	}

	// If that fails, try to parse as wrapped response
	var wrappedResponse api.FileEntriesResponse
	if err := json.Unmarshal(resultBytes, &wrappedResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response as wrapped object: %w", err)
	}

	// Use entries from data field if available, otherwise from entries field
	if len(wrappedResponse.Data) > 0 {
		entries = wrappedResponse.Data
		fs.Debugf(f, "Parsed as wrapped response with data field, found %d entries", len(entries))
	} else {
		entries = wrappedResponse.Entries
		fs.Debugf(f, "Parsed as wrapped response with entries field, found %d entries", len(entries))
	}

	return entries, nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	// Resolve directory relative to filesystem root
	targetDir := dir
	if dir == "" && f.root != "" {
		targetDir = f.root
	} else if dir != "" && f.root != "" {
		targetDir = path.Join(f.root, dir)
	}
	
	fs.Debugf(f, "List: dir='%s', targetDir='%s'", dir, targetDir)
	
	parentID, err := f.getParentID(ctx, targetDir)
	if err != nil {
		return nil, err
	}

	items, err := f.listAll(ctx, parentID)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		remote := item.Name
		if dir != "" {
			remote = path.Join(dir, item.Name)
		}

		switch item.Type {
		case "folder":
			d := fs.NewDir(remote, item.CreatedAt)
			entries = append(entries, d)
		default:
			o := &Object{
				fs:          f,
				remote:      remote,
				hasMetaData: true,
				size:        item.FileSize,
				modTime:     item.UpdatedAt,
				id:          item.ID,
				parentID:    item.ParentID,
				mimeType:    item.Mime,
				hash:        item.Hash,
				downloadURL: item.URL,
			}
			entries = append(entries, o)
		}
	}

	return entries, nil
}

// NewObject finds the Object at remote
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// newObjectWithInfo creates an object with the given info if possible
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.FileEntry) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}

	if info != nil {
		o.hasMetaData = true
		o.size = info.FileSize
		o.modTime = info.UpdatedAt
		o.id = info.ID
		o.parentID = info.ParentID
		o.mimeType = info.Mime
		o.hash = info.Hash
		o.downloadURL = info.URL
	} else {
		err := o.readMetaData(ctx)
		if err != nil {
			return nil, err
		}
	}

	return o, nil
}

// Put creates a new object or updates an existing one
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.NewObject(ctx, src.Remote())
	if err == nil {
		err = existingObj.Update(ctx, in, src, options...)
		if err != nil {
			return nil, err
		}
		return existingObj, nil
	}
	// Object doesn't exist, create new one
	return f.PutUnchecked(ctx, in, src, options...)
}

// PutUnchecked uploads the object
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()

	// Get parent directory - this is relative to the filesystem root
	dir, fileName := path.Split(remote)
	dir = strings.TrimSuffix(dir, "/")
	
	// If dir is empty, we're uploading to the filesystem root
	// If filesystem root is not empty, we need to resolve that
	targetDir := dir
	if dir == "" && f.root != "" {
		targetDir = f.root
	} else if dir != "" && f.root != "" {
		targetDir = path.Join(f.root, dir)
	}
	
	fs.Debugf(f, "Upload target: remote='%s', dir='%s', targetDir='%s', fileName='%s'", remote, dir, targetDir, fileName)
	
	parentID, err := f.getParentID(ctx, targetDir)
	if err != nil {
		return nil, err
	}

	// Check if we should use S3 multipart upload
	if size >= 0 && size > int64(f.opt.ChunkSize) {
		fs.Debugf(f, "File size %d exceeds chunk size %d, using S3 multipart upload", size, f.opt.ChunkSize)
		mu := f.newMultipartUpload(ctx, remote, src, parentID, options...)
		return mu.upload(ctx, in)
	}

	// Upload the file normally
	modTime := src.ModTime(ctx)
	o, err := f.putUnchecked(ctx, in, fileName, parentID, size, modTime, options...)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// putUnchecked uploads the object with the given filename and parent
func (f *Fs) putUnchecked(ctx context.Context, in io.Reader, fileName string, parentID *int, size int64, modTime time.Time, options ...fs.OpenOption) (*Object, error) {
	// Create multipart form
	formReader, contentType, _, err := rest.MultipartUpload(ctx, in, nil, "file", fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to make multipart upload: %w", err)
	}

	opts := rest.Opts{
		Method:      "POST",
		Path:        "/uploads",
		ContentType: contentType,
		Body:        formReader,
	}

	// Add parentId if specified
	if parentID != nil {
		opts.Parameters = url.Values{
			"parentId": {strconv.Itoa(*parentID)},
		}
		fs.Debugf(f, "Uploading '%s' to parentID: %d", fileName, *parentID)
	} else {
		fs.Debugf(f, "Uploading '%s' to root", fileName)
	}

	var response api.UploadResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &response)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	fs.Debugf(f, "Upload response: %+v", response)

	// Create object from response
	obj, err := f.newObjectWithInfo(ctx, fileName, &response.FileEntry)
	if err != nil {
		return nil, err
	}
	return obj.(*Object), nil
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	fs.Debugf(f, "Mkdir called with dir: '%s'", dir)
	
	// If dir is empty, we need to create the root directory (f.root)
	targetDir := dir
	if dir == "" && f.root != "" {
		targetDir = f.root
		fs.Debugf(f, "Empty dir, using root: '%s'", targetDir)
	}
	
	// If both dir and root are empty, nothing to create (root directory)
	if targetDir == "" {
		fs.Debugf(f, "No directory to create (root)")
		return nil
	}
	
	// Split path and create directories recursively
	parts := strings.Split(strings.Trim(targetDir, "/"), "/")
	fs.Debugf(f, "Split into parts: %v", parts)
	
	var parentID *int

	for i, part := range parts {
		if part == "" {
			continue
		}

		fs.Debugf(f, "Processing part %d: '%s'", i, part)

		// Check if directory already exists
		entries, err := f.listAll(ctx, parentID)
		if err != nil {
			return err
		}

		found := false
		for _, entry := range entries {
			if entry.Name == part && entry.Type == "folder" {
				parentID = &entry.ID
				found = true
				fs.Debugf(f, "Found existing folder '%s' with ID: %d", part, entry.ID)
				break
			}
		}

		if !found {
			// Create the directory
			fs.Debugf(f, "Creating new folder '%s' with parentID: %v", part, parentID)
			parentID, err = f.createDir(ctx, part, parentID)
			if err != nil {
				return err
			}
			fs.Debugf(f, "Successfully created folder '%s' with ID: %d", part, *parentID)
		}
	}

	return nil
}

// createDir creates a single directory
func (f *Fs) createDir(ctx context.Context, name string, parentID *int) (*int, error) {
	request := api.CreateFolderRequest{
		Name:     name,
		ParentID: parentID,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/folders",
	}

	fs.Debugf(f, "Creating folder: %s with parentID: %v", name, parentID)

	var response api.CreateFolderResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &request, &response)
		fs.Debugf(f, "Folder creation response: %+v", response)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	fs.Debugf(f, "Created folder with ID: %d", response.Folder.ID)
	return &response.Folder.ID, nil
}

// Rmdir removes the directory if empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	parentID, err := f.getParentID(ctx, dir)
	if err != nil {
		return err
	}

	if parentID == nil {
		return fmt.Errorf("cannot remove root directory")
	}

	// Check if directory is empty
	entries, err := f.listAll(ctx, parentID)
	if err != nil {
		return err
	}

	if len(entries) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	// Delete the directory
	return f.deleteEntry(ctx, []string{strconv.Itoa(*parentID)}, false)
}

// deleteEntry deletes entries by ID
func (f *Fs) deleteEntry(ctx context.Context, entryIDs []string, deleteForever bool) error {
	// Convert string IDs to integers
	intIDs := make([]int, len(entryIDs))
	for i, id := range entryIDs {
		intID, err := strconv.Atoi(id)
		if err != nil {
			return fmt.Errorf("invalid entry ID '%s': %w", id, err)
		}
		intIDs[i] = intID
	}

	request := api.DeleteEntriesRequest{
		EntryIDs:      intIDs,
		DeleteForever: deleteForever,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/file-entries/delete", // Try a POST method for deletion
	}

	fs.Debugf(f, "Deleting entries: %v, deleteForever: %v", intIDs, deleteForever)

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &request, nil)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to delete entries: %w", err)
	}

	return nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Object = (*Object)(nil)
)
