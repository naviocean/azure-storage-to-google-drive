# Azure Storage Backup to Google Drive

Automated system to backup Azure Storage containers to Google Drive and restore when needed.

## Prerequisites

1. Azure Storage Account:
- Account Name
- Account Key
- Container Name(s)

2. Google Drive:
- Shared Drive set up
- OAuth 2.0 Client ID credentials

## Setup Google Drive Access

### 1. Google Drive Credentials Setup

To create `credentials.json`:

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing project
3. Enable Google Drive API:
   - Go to "APIs & Services" > "Library"
   - Search for "Google Drive API"
   - Click "Enable"

4. Configure OAuth Consent Screen:
   - Go to "APIs & Services" > "OAuth consent screen"
   - Select "External" user type
   - Fill in the application name and other required fields
   - Add scopes: `https://www.googleapis.com/auth/drive.file` and `https://www.googleapis.com/auth/drive`
   - Add your email in test users

5. Create OAuth Client ID:
   - Go to "APIs & Services" > "Credentials"
   - Click "Create Credentials" > "OAuth client ID"
   - Select "Desktop application" as application type
   - Give it a name
   - Click "Create"

6. Download Credentials:
   - After creation, click the download button (JSON format)
   - Rename the downloaded file to `credentials.json`
   - Place it in the project root directory

### 2. Azure Storage Setup

1. Create Azure Storage account or use existing one
2. Get the following information:
   - Storage Account Name
   - Storage Account Key
   - Container Name

3. For restore service (target storage):
   - Create another storage account if needed
   - Get the same information as above

### 3. Environment Configuration

1. Environment setup:
```bash
cp .env.example .env
```

2. Configure .env:
```bash
# Source Azure (for backup)
AZURE_ACCOUNT_NAME=source_account
AZURE_ACCOUNT_KEY=source_key
AZURE_CONTAINER_NAME=ALL  # "ALL" or specific container

# Target Azure (for restore)
TARGET_AZURE_ACCOUNT_NAME=target_account
TARGET_AZURE_ACCOUNT_KEY=target_key
TARGET_AZURE_CONTAINER_NAME=ALL

# Google Drive
GOOGLE_SHARED_DRIVE_ID=your_drive_id
GOOGLE_FOLDER_ID=optional_folder_id

# Backup Schedule (cron format)
BACKUP_SCHEDULE="0 1 * * *"  # 1 AM daily
BACKUP_RETENTION_DAYS=7
```

### 4. Generate Google Drive Token

```bash
# Run token generator
docker-compose run --rm token-generator

# Follow the instructions:
# 1. Open provided URL in browser
# 2. Login and authorize the application
# 3. Copy authorization code
# 4. Paste code back in terminal
```

### 5. Start Backup Service

```bash
# Run backup service (follows schedule)
docker-compose up -d backup-service

# Check logs
docker-compose logs -f backup-service
```

### 6. Restore When Needed

```bash
# Latest backup
docker-compose run --rm restore-service

# Specific date
docker-compose run --rm restore-service -date="2023-11-14"

# Check logs
docker-compose logs restore-service
```

## Backup Features

- Incremental backup (only changed files)
- Multiple containers support
- Compression before upload
- Retention policy
- Progress tracking
- Detailed logging
- Automatic cleanup

## Restore Features

- Full or specific container restore
- Date-based restore
- Automatic container creation
- Concurrent file processing
- Progress monitoring
- Atomic operations

## Logging

```bash
# Set log level in .env:
LOG_LEVEL=debug  # debug, info, warn, error

# View logs:
docker-compose logs -f backup-service
docker-compose logs -f restore-service
```

## Best Practices

1. Security:
- Use separate storage accounts for backup/restore
- Rotate access keys regularly
- Secure credentials.json and token.json

2. Monitoring:
- Check logs regularly
- Monitor disk space
- Verify backup success

3. Testing:
- Test restore process periodically
- Verify file integrity
- Check backup retention

## Troubleshooting

## Troubleshooting

1. Token Issues:
```bash
# Regenerate token
rm token.json
docker-compose run --rm token-generator
```

2. Azure Issues:
- Verify account credentials
- Check container permissions
- Ensure sufficient quota

3. Backup Failures:
- Check source storage access
- Verify sufficient disk space
- Review error logs

## License

MIT