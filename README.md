# Cloud Backup System

A system for backing up Azure Storage to Google Drive and restoring when needed.

## Components

1. **token-generator**: Generates Google Drive OAuth token
2. **backup-service**: Automated backup from Azure Storage to Google Drive
3. **restore-service**: Restore from Google Drive to target Azure Storage

## Setup

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

```bash
# Copy example environment file
cp .env.example .env

# Edit .env with your configuration
vim .env
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
# Start backup service
docker-compose up -d backup-service
```

### 6. Restore When Needed

```bash
# Restore latest backup
docker-compose run --rm restore-service

# Or restore from specific date
docker-compose run --rm restore-service ./restore-service -date="2023-11-14"
```

## Configuration

### Environment Variables

See `.env.example` for all available configuration options.

### Backup Schedule

Default schedule is daily at 1 AM. Modify `BACKUP_SCHEDULE` to change.

### Retention Policy

Backups are kept for 7 days by default. Modify `BACKUP_RETENTION_DAYS` to change.

## Architecture

- Uses incremental backup to minimize storage and bandwidth
- Compresses files before upload
- Concurrent operations for better performance
- Automatic cleanup of old backups
- Health monitoring

## Troubleshooting

### Common Issues

1. Token Generation Fails:
   - Ensure `credentials.json` has correct format
   - Check if Google Drive API is enabled
   - Verify OAuth consent screen configuration
   - Make sure you're using correct Google account

2. Backup Service Issues:
   - Check Azure credentials
   - Verify container permissions
   - Check available disk space
   - Review service logs

3. Restore Service Issues:
   - Verify target Azure storage credentials
   - Check Google Drive permissions
   - Ensure sufficient disk space for temporary files

### Logs

```bash
# View backup service logs
docker-compose logs -f backup-service

# View restore service logs
docker-compose logs restore-service
```

## Security Notes

1. Credentials:
   - Keep `credentials.json` and `token.json` secure
   - Don't commit these files to git
   - Use appropriate file permissions

2. Azure Storage:
   - Use separate storage accounts for source and target
   - Configure appropriate CORS settings if needed
   - Use least privilege access principles

## License

MIT