// Package api defines types for interacting with the FolderFort API.
package api

import (
	"time"
)

// FileEntry represents a file or folder entry in FolderFort
type FileEntry struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	FileName    string      `json:"file_name"`
	FileSize    int64       `json:"file_size"`
	ParentID    *int        `json:"parent_id"`
	Parent      *FileEntry  `json:"parent,omitempty"`
	Thumbnail   interface{} `json:"thumbnail"` // Can be string or boolean
	Mime        string      `json:"mime"`
	URL         string      `json:"url"`
	Hash        string      `json:"hash"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	DeletedAt   *time.Time  `json:"deleted_at"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	Path        string      `json:"path"`
	Users       []User      `json:"users,omitempty"`
}

// User represents a user in FolderFort
type User struct {
	ID          int       `json:"id"`
	AccessToken string    `json:"access_token,omitempty"`
	DisplayName string    `json:"display_name"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	Email       string    `json:"email"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ShareableLink represents a shareable link
type ShareableLink struct {
	ID            int        `json:"id"`
	Hash          string     `json:"hash"`
	Password      string     `json:"password"`
	UserID        int        `json:"user_id"`
	EntryID       int        `json:"entry_id"`
	Entry         *FileEntry `json:"entry,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at"`
	AllowEdit     bool       `json:"allow_edit"`
	AllowDownload bool       `json:"allow_download"`
}

// Tag represents a tag (for starring)
type Tag struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Response wrappers
type UploadResponse struct {
	Status    string    `json:"status"`
	FileEntry FileEntry `json:"fileEntry"`
}

type FileEntriesResponse struct {
	Status     string          `json:"status,omitempty"`
	Data       []FileEntry     `json:"data,omitempty"`
	Entries    []FileEntry     `json:"entries,omitempty"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

type PaginationInfo struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
}

type CreateFolderResponse struct {
	Status string    `json:"status"`
	Folder FileEntry `json:"folder"`
}

type ShareableLinkResponse struct {
	Status         string        `json:"status"`
	Link           ShareableLink `json:"link"`
	FolderChildren []FileEntry   `json:"folderChildren,omitempty"`
}

type StarResponse struct {
	Status string `json:"status"`
	Tag    Tag    `json:"tag"`
}

type LoginResponse struct {
	Status string `json:"status"`
	User   User   `json:"user"`
}

type RegisterResponse struct {
	Status string `json:"status"`
	User   User   `json:"user"`
}

type ShareResponse struct {
	Status string `json:"status"`
	Users  []User `json:"users"`
}

// Request types
type LoginRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	TokenName string `json:"token_name"`
}

type RegisterRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	TokenName string `json:"token_name"`
}

type UpdateEntryRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type CreateFolderRequest struct {
	Name     string `json:"name"`
	ParentID *int   `json:"parentId"`
}

type MoveEntriesRequest struct {
	EntryIDs      []int `json:"entryIds"`
	DestinationID *int  `json:"destinationId"`
}

type DeleteEntriesRequest struct {
	EntryIDs      []int `json:"entryIds"`
	DeleteForever bool  `json:"deleteForever"`
	EmptyTrash    bool  `json:"emptyTrash"`
}

// CopyEntriesRequest is used to duplicate entries
type CopyEntriesRequest struct {
	EntryIDs      []int `json:"entryIds"`
	DestinationID *int  `json:"destinationId"`
}

// CopyEntriesResponse is the response from duplicating entries
type CopyEntriesResponse struct {
	Status  string      `json:"status"`
	Entries []FileEntry `json:"entries"`
}

type RestoreEntriesRequest struct {
	EntryIDs []int `json:"entryIds"`
}

type StarEntriesRequest struct {
	EntryIDs []int `json:"entryIds"`
}

type ShareEntryRequest struct {
	Emails      []string `json:"emails"`
	Permissions []string `json:"permissions"`
}

type ChangePermissionsRequest struct {
	UserID      int      `json:"userId"`
	Permissions []string `json:"permissions"`
}

type UnshareEntryRequest struct {
	UserID int `json:"userId"`
}

type CreateShareableLinkRequest struct {
	Password      string     `json:"password,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	AllowEdit     bool       `json:"allow_edit"`
	AllowDownload bool       `json:"allow_download"`
}

type UpdateShareableLinkRequest struct {
	Password      string     `json:"password,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	AllowEdit     bool       `json:"allow_edit"`
	AllowDownload bool       `json:"allow_download"`
}

// Error response
type ErrorResponse struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Errors  map[string]interface{} `json:"errors,omitempty"`
}

// SpaceUsageResponse represents the response from space usage API
type SpaceUsageResponse struct {
	Status    string    `json:"status"`
	Used      int64     `json:"used"`
	Available int64     `json:"available"`
	SEO       *struct{} `json:"seo"`
}

// S3 Multipart Upload API types

// CreateMultipartUploadRequest starts a multipart upload
type CreateMultipartUploadRequest struct {
	Filename     string `json:"filename"`
	Mime         string `json:"mime"`
	Size         int64  `json:"size"`
	Extension    string `json:"extension"`
	WorkspaceID  int    `json:"workspaceId"`
	ParentID     int    `json:"parentId"`
	RelativePath string `json:"relativePath"`
	Disk         string `json:"disk"`
}

// CreateMultipartUploadResponse contains the upload ID and key
type CreateMultipartUploadResponse struct {
	Status   string `json:"status"`
	Key      string `json:"key"`
	UploadID string `json:"uploadId"`
	ACL      string `json:"acl"`
}

// BatchSignPartURLsRequest requests signed URLs for uploading parts
type BatchSignPartURLsRequest struct {
	PartNumbers []int  `json:"partNumbers"`
	UploadID    string `json:"uploadId"`
	Key         string `json:"key"`
}

// PartURLInfo contains the signed URL for a part
type PartURLInfo struct {
	PartNumber int    `json:"partNumber"`
	URL        string `json:"url"`
}

// BatchSignPartURLsResponse contains signed URLs for parts
type BatchSignPartURLsResponse struct {
	Status string        `json:"status"`
	URLs   []PartURLInfo `json:"urls"`
}

// CompleteMultipartUploadRequest completes a multipart upload
type CompleteMultipartUploadRequest struct {
	Key      string          `json:"key"`
	UploadID string          `json:"uploadId"`
	Parts    []CompletedPart `json:"parts"`
}

// CompletedPart represents a completed upload part
type CompletedPart struct {
	PartNumber int    `json:"PartNumber"`
	ETag       string `json:"ETag"`
}

// CompleteMultipartUploadResponse contains the final location
type CompleteMultipartUploadResponse struct {
	Status   string `json:"status"`
	Location string `json:"location"`
}

// CreateS3EntryRequest creates a file entry from uploaded S3 file
type CreateS3EntryRequest struct {
	WorkspaceID     int    `json:"workspaceId"`
	ParentID        string `json:"parentId"`
	RelativePath    string `json:"relativePath"`
	Disk            string `json:"disk"`
	ClientMime      string `json:"clientMime"`
	ClientName      string `json:"clientName"`
	Filename        string `json:"filename"`
	Size            int64  `json:"size"`
	ClientExtension string `json:"clientExtension"`
}

// CreateS3EntryResponse contains the created file entry
type CreateS3EntryResponse struct {
	Status    string    `json:"status"`
	FileEntry FileEntry `json:"fileEntry"`
}
