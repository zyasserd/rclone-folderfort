# FolderFort Backend - Maintainers Notes

## Outstanding Issues & TODOs

### Failing Tests
- `fs/sync`:
  - `TestSyncBackupDirWithSuffix`
  - `TestSyncBackupDirWithSuffixKeepExtension`

### Unimplemented Features
- Workspaces
- PublicLinker
- ListR
    - FolderFort API lacks native recursive listing (ListR), though /drive/file-entries can list multiple folders with pagination.
    - Without ListR, backup tools like Kopia face slow file listing because each folder listing takes ~1 second, and Kopia doesn’t support concurrent listing for listing blobs afaik.
        - WORKAROUND: Reduce or eliminate sharding to limit the number of folders, balancing between folder size and request overhead until native ListR is available.

### WebDAV Issues
- No server-side copy
    - Copy operations are performed client-side, making them slower and less efficient.
- Canceling copy is ineffective
    - Although canceling a copy operation appears to stop it, the process continues running in the background.
- Deleting non-empty directories fails
    - Attempts to delete directories that contain files or subfolders result in errors.


## Testing the Backend

This backend includes a helper script: [`tests.sh`](./tests.sh)

**Usage:**

```sh
./tests.sh <remote_name>
```

- `<remote_name>` is required (e.g., `folderfort_test:` or any configured remote).
- The script runs unit and integration tests for the backend and saves diagnostics in a `diagnostics/<git-hash>/` directory.
- Example:
  ```sh
  ./tests.sh folderfort_test:
  ```
