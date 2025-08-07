// Package api defines types for interacting with the FolderFort API.
package api

import (
	"time"
)

// FileEntry represents a file or folder entry in FolderFort
type FileEntry struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ParentID    *int   `json:"parent_id"`
	Parent      *FileEntry `json:"parent,omitempty"`
	Thumbnail   interface{} `json:"thumbnail"` // Can be string or boolean
	Mime        string `json:"mime"`
	URL         string `json:"url"`
	Hash        string `json:"hash"`
	Type        string `json:"type"`
	Description string `json:"description"`
	DeletedAt   *time.Time `json:"deleted_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Path        string     `json:"path"`
	Users       []User     `json:"users,omitempty"`
}

// User represents a user in FolderFort
type User struct {
	ID          int    `json:"id"`
	AccessToken string `json:"access_token,omitempty"`
	DisplayName string `json:"display_name"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Email       string `json:"email"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ShareableLink represents a shareable link
type ShareableLink struct {
	ID            int       `json:"id"`
	Hash          string    `json:"hash"`
	Password      string    `json:"password"`
	UserID        int       `json:"user_id"`
	EntryID       int       `json:"entry_id"`
	Entry         *FileEntry `json:"entry,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at"`
	AllowEdit     bool      `json:"allow_edit"`
	AllowDownload bool      `json:"allow_download"`
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
	Status     string            `json:"status,omitempty"`
	Data       []FileEntry       `json:"data,omitempty"`
	Entries    []FileEntry       `json:"entries,omitempty"`
	Pagination *PaginationInfo   `json:"pagination,omitempty"`
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
	Status          string        `json:"status"`
	Link            ShareableLink `json:"link"`
	FolderChildren  []FileEntry   `json:"folderChildren,omitempty"`
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
