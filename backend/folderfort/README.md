# TODO
- failed tests
    - fs/sync TestSyncBackupDirWithSuffix
    - fs/sync TestSyncBackupDirWithSuffixKeepExtension
    - fs/sync TestSyncConcurrentTruncate

- unimplemented
    - Workspaces
    - PublicLinker
    - ListR
        - it's not supported natively but you can get better performance that list
            - by `/drive/file-entries` using multiple folders in one request
                - figure out pagination
        - otherwise kopia's list blobs is taking ~1 s/blob
            - took me 45 mins for a 50GB repo

- webdav issues
    - copying
        - doesn't run server side

    - cancelling copy doesn't work
        - it shows it has been cancelled, but still runs in the background

    - deleting non empty directory error

