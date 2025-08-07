# FolderFort Backend for rclone

This backend provides integration with FolderFort cloud storage systems.

## Configuration

To configure the FolderFort backend, you'll need:

1. **URL**: The URL of your FolderFort instance (e.g., `https://yoursite.com`)
2. **Access Token**: An access token from your FolderFort account

### Getting an Access Token

You can get an access token in two ways:

1. **From Account Settings**: Log into your FolderFort web interface and go to Account Settings to generate an API token
2. **Via Login API**: Use the `/auth/login` endpoint with your email and password to get a token

### Example Configuration

```
rclone config

n) New remote
name> myfolderfort
Type of storage> folderfort
URL of your FolderFort instance> https://yoursite.com
Access token> your_access_token_here
Workspace ID> 0
```

## Features

This backend supports:

- ✅ Reading files and directories
- ✅ Writing files
- ✅ Creating directories
- ✅ Deleting files and directories
- ✅ Moving files (via copy + delete)
- ✅ File metadata (size, modification time)
- ❌ Setting modification times (not supported by FolderFort API)
- ❌ File hashing (not provided by FolderFort API)

## Usage Examples

```bash
# List files
rclone ls myfolderfort:

# Copy local directory to FolderFort
rclone copy /local/path myfolderfort:remote/path

# Sync directories
rclone sync myfolderfort:source myfolderfort:dest

# Mount FolderFort as a local filesystem
rclone mount myfolderfort: /mnt/folderfort
```

## Limitations

- File hashing is not supported as the FolderFort API doesn't provide file hashes
- Setting modification times is not supported by the FolderFort API
- Large file uploads may need to be chunked (not yet implemented)

## API Compatibility

This backend is designed to work with FolderFort systems that implement the API specification provided. It uses:

- Bearer token authentication
- RESTful HTTP API endpoints
- JSON request/response format
- Multipart file uploads

## Error Handling

The backend includes appropriate error handling for common scenarios:

- Authentication errors (401/403)
- Not found errors (404)
- Rate limiting (429)
- Server errors (5xx)

## Testing

To run tests:

```bash
# Set up test environment
export RCLONE_CONFIG_FOLDERFORT_TYPE=folderfort
export RCLONE_CONFIG_FOLDERFORT_URL=https://your-test-instance.com
export RCLONE_CONFIG_FOLDERFORT_ACCESS_TOKEN=your_test_token

# Run tests
go test ./backend/folderfort/...
```
