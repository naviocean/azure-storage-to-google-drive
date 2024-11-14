# Token Generator

A utility service to generate and manage Google Drive OAuth tokens.

## Features

- Generates OAuth tokens for Google Drive access
- Supports shared drive access
- Validates and refreshes existing tokens
- Secure token storage
- Interactive token generation process

## Usage

1. Place your `credentials.json` file in the root directory
2. Run the token generator:

```bash
docker-compose run --rm token-generator
```

3. Follow the prompts:
   - Open the provided URL in your browser
   - Login to Google account
   - Grant requested permissions
   - Copy the authorization code
   - Paste the code back into the terminal

## Environment Variables

- `GOOGLE_CREDENTIALS_PATH`: Path to credentials file (default: `/app/credentials.json`)
- `GOOGLE_TOKEN_PATH`: Path to save token (default: `/app/token.json`)

## Security Notes

- Token file permissions are set to 600 (read/write for owner only)
- Refresh tokens are always requested and validated
- Existing tokens are validated and refreshed if possible