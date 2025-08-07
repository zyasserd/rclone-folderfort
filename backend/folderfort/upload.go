// Upload large files for folderfort using S3 multipart API
package folderfort

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/rclone/rclone/backend/folderfort/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/sync/errgroup"
)

// multipartUpload handles S3-style multipart uploads to FolderFort
type multipartUpload struct {
	f         *Fs
	remote    string
	src       fs.ObjectInfo
	parentID  *int
	chunkSize int64
	options   []fs.OpenOption
	uploadID  string
	key       string
}

// newMultipartUpload creates a new S3 multipart upload
func (f *Fs) newMultipartUpload(ctx context.Context, remote string, src fs.ObjectInfo, parentID *int, options ...fs.OpenOption) *multipartUpload {
	return &multipartUpload{
		f:         f,
		remote:    remote,
		src:       src,
		parentID:  parentID,
		chunkSize: int64(f.opt.ChunkSize),
		options:   options,
	}
}

// upload performs a multipart upload using S3 API
func (mu *multipartUpload) upload(ctx context.Context, in io.Reader) (*Object, error) {
	size := mu.src.Size()

	// Calculate number of parts (S3 minimum part size is 5MB except for the last part)
	minPartSize := int64(5 * 1024 * 1024) // 5MB
	if mu.chunkSize < minPartSize {
		mu.chunkSize = minPartSize
	}

	numParts := size / mu.chunkSize
	if size%mu.chunkSize != 0 {
		numParts++
	}

	fs.Debugf(mu.f, "Starting S3 multipart upload of %s: size=%d, parts=%d, partSize=%d",
		mu.remote, size, numParts, mu.chunkSize)

	// Step 1: Start multipart upload
	err := mu.startMultipartUpload(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start multipart upload: %w", err)
	}

	// Step 2: Get signed URLs for all parts
	partNumbers := make([]int, numParts)
	for i := range partNumbers {
		partNumbers[i] = i + 1 // S3 part numbers start from 1
	}

	signedURLs, err := mu.getSignedPartURLs(ctx, partNumbers)
	if err != nil {
		return nil, fmt.Errorf("failed to get signed URLs: %w", err)
	}

	// Step 3: Upload parts in parallel
	completedParts, err := mu.uploadParts(ctx, in, signedURLs, size)
	if err != nil {
		return nil, fmt.Errorf("failed to upload parts: %w", err)
	}

	// Step 4: Complete multipart upload
	finalObj, err := mu.completeMultipartUpload(ctx, completedParts)
	if err != nil {
		return nil, fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	fs.Debugf(mu.f, "Completed S3 multipart upload of %s", mu.remote)
	return finalObj, nil
}

// startMultipartUpload initiates a multipart upload
func (mu *multipartUpload) startMultipartUpload(ctx context.Context) error {
	// Extract filename from remote path
	_, fileName := path.Split(mu.remote)

	// Get file extension
	ext := path.Ext(fileName)
	if ext != "" {
		ext = ext[1:] // Remove the dot
	}

	// Determine parent ID - if nil, use 0 for root
	parentID := 0
	if mu.parentID != nil {
		parentID = *mu.parentID
	}

	request := api.CreateMultipartUploadRequest{
		Filename:     fileName,
		Mime:         "application/octet-stream", // Default MIME type
		Size:         mu.src.Size(),
		Extension:    ext,
		WorkspaceID:  mu.f.opt.WorkspaceID,
		ParentID:     parentID,
		RelativePath: "",
		Disk:         "uploads",
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/s3/multipart/create",
	}

	var response api.CreateMultipartUploadResponse
	err := mu.f.pacer.Call(func() (bool, error) {
		resp, err := mu.f.srv.CallJSON(ctx, &opts, &request, &response)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}

	mu.uploadID = response.UploadID
	mu.key = response.Key

	fs.Debugf(mu.f, "Started multipart upload: uploadID=%s, key=%s", mu.uploadID, mu.key)
	return nil
}

// getSignedPartURLs gets signed URLs for uploading parts
func (mu *multipartUpload) getSignedPartURLs(ctx context.Context, partNumbers []int) ([]api.PartURLInfo, error) {
	request := api.BatchSignPartURLsRequest{
		PartNumbers: partNumbers,
		UploadID:    mu.uploadID,
		Key:         mu.key,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/s3/multipart/batch-sign-part-urls",
	}

	var response api.BatchSignPartURLsResponse
	err := mu.f.pacer.Call(func() (bool, error) {
		resp, err := mu.f.srv.CallJSON(ctx, &opts, &request, &response)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	fs.Debugf(mu.f, "Got %d signed URLs for parts", len(response.URLs))
	return response.URLs, nil
}

// uploadParts uploads all parts to their signed URLs
func (mu *multipartUpload) uploadParts(ctx context.Context, in io.Reader, signedURLs []api.PartURLInfo, totalSize int64) ([]api.CompletedPart, error) {
	// unwrap the accounting from the input
	in, wrap := accounting.UnWrap(in)

	var completedParts []api.CompletedPart
	var mu_lock sync.Mutex

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(mu.f.opt.UploadConcurrency)

	// Read all data into memory first for parallel uploads
	allData, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read upload data: %w", err)
	}

	for i, urlInfo := range signedURLs {
		i := i
		urlInfo := urlInfo

		g.Go(func() error {
			// Calculate part data
			partStart := int64(i) * mu.chunkSize
			partEnd := partStart + mu.chunkSize
			if partEnd > totalSize {
				partEnd = totalSize
			}
			partData := allData[partStart:partEnd]

			fs.Debugf(mu.f, "Uploading part %d: %d bytes", urlInfo.PartNumber, len(partData))

			// Upload part to signed URL
			etag, err := mu.uploadPart(gCtx, urlInfo.URL, partData, wrap)
			if err != nil {
				return fmt.Errorf("failed to upload part %d: %w", urlInfo.PartNumber, err)
			}

			// Add to completed parts
			mu_lock.Lock()
			completedParts = append(completedParts, api.CompletedPart{
				PartNumber: urlInfo.PartNumber,
				ETag:       etag,
			})
			mu_lock.Unlock()

			fs.Debugf(mu.f, "Completed part %d with ETag: %s", urlInfo.PartNumber, etag)
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		return nil, err
	}

	// Sort completed parts by part number (S3 requires this)
	sort.Slice(completedParts, func(i, j int) bool {
		return completedParts[i].PartNumber < completedParts[j].PartNumber
	})

	return completedParts, nil
}

// uploadPart uploads a single part to a signed URL
func (mu *multipartUpload) uploadPart(ctx context.Context, signedURL string, data []byte, wrap accounting.WrapFn) (string, error) {
	dataSize := int64(len(data))

	fs.Debugf(mu.f, "Uploading to URL: %s with %d bytes", signedURL, len(data))

	req, err := http.NewRequestWithContext(ctx, "PUT", signedURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	// Set Content-Length explicitly
	req.Header.Set("Content-Length", strconv.FormatInt(dataSize, 10))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = dataSize // Also set the ContentLength field directly

	fs.Debugf(mu.f, "Request headers: %v, ContentLength: %d", req.Header, req.ContentLength)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	fs.Debugf(mu.f, "Upload response status: %d, headers: %v", resp.StatusCode, resp.Header)

	if resp.StatusCode != 200 {
		// Read response body for more details
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload part failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Get ETag from response headers
	etag := resp.Header.Get("ETag")
	if etag == "" {
		return "", fmt.Errorf("no ETag in response")
	}

	// Keep quotes in ETag as they appear in the response
	// The API expects them with quotes
	return etag, nil
} // completeMultipartUpload completes the multipart upload
func (mu *multipartUpload) completeMultipartUpload(ctx context.Context, parts []api.CompletedPart) (*Object, error) {
	request := api.CompleteMultipartUploadRequest{
		Key:      mu.key,
		UploadID: mu.uploadID,
		Parts:    parts,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/s3/multipart/complete",
	}

	var response api.CompleteMultipartUploadResponse
	err := mu.f.pacer.Call(func() (bool, error) {
		resp, err := mu.f.srv.CallJSON(ctx, &opts, &request, &response)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	fs.Debugf(mu.f, "Completed multipart upload, location: %s", response.Location)

	// Step 5: Create the file entry in FolderFort's system
	fileEntry, err := mu.createS3Entry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create file entry: %w", err)
	}

	fs.Debugf(mu.f, "Created file entry: %s (ID: %d, size: %d)", fileEntry.Name, fileEntry.ID, fileEntry.FileSize)

	// Create the object from the file entry
	obj, err := mu.f.newObjectWithInfo(ctx, mu.remote, fileEntry)
	if err != nil {
		return nil, err
	}
	return obj.(*Object), nil
}

// createS3Entry creates a file entry from the uploaded S3 file
func (mu *multipartUpload) createS3Entry(ctx context.Context) (*api.FileEntry, error) {
	// Extract filename and extension from remote path
	_, fileName := path.Split(mu.remote)
	ext := path.Ext(fileName)
	if ext != "" {
		ext = ext[1:] // Remove the dot
	}

	// Determine parent ID string - if nil, use empty string for root
	parentIDStr := ""
	if mu.parentID != nil {
		parentIDStr = fmt.Sprintf("%d", *mu.parentID)
	}

	// Extract the UUID from the key (everything after "uploads/")
	// The key format is: "uploads/uuid/uuid"
	keyParts := strings.Split(mu.key, "/")
	if len(keyParts) < 2 {
		return nil, fmt.Errorf("invalid key format: %s", mu.key)
	}
	filename := keyParts[len(keyParts)-1] // Get the last part (UUID)

	request := api.CreateS3EntryRequest{
		WorkspaceID:     mu.f.opt.WorkspaceID,
		ParentID:        parentIDStr,
		RelativePath:    "",
		Disk:            "uploads",
		ClientMime:      "application/octet-stream",
		ClientName:      fileName,
		Filename:        filename,
		Size:            mu.src.Size(),
		ClientExtension: ext,
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/s3/entries",
	}

	var response api.CreateS3EntryResponse
	err := mu.f.pacer.Call(func() (bool, error) {
		resp, err := mu.f.srv.CallJSON(ctx, &opts, &request, &response)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	return &response.FileEntry, nil
}

// generateUUID generates a proper UUID for uploads
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
