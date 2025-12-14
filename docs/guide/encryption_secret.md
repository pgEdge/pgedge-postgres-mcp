# Encryption Secret File

The server uses a separate encryption secret file to store the encryption key used for password encryption. This file contains a 256-bit AES encryption key used to encrypt and decrypt database passwords.

**Default Location**: `pgedge-mcp-server.secret` in the same directory as the binary

**Configuration Priority** (highest to lowest):

1. Environment variable: `PGEDGE_SECRET_FILE=/path/to/secret`
2. Configuration file: `secret_file: /path/to/secret`
3. Default: `pgedge-mcp-server.secret` (same directory as binary)

## Auto-Generation

The secret file is automatically generated on first run if it doesn't exist:

```bash
# First run - secret file will be auto-generated
./bin/pgedge-mcp-server

# Output:
# Generating new encryption key at /path/to/pgedge-mcp-server.secret
# Encryption key saved successfully
```

## File Format

The secret file contains a base64-encoded 256-bit encryption key:

```
base64_encoded_32_byte_key_here==
```

### Security Considerations

- **File Permissions**:
    - The secret file is created with `0600` permissions (owner read/write only)
    - The server will **refuse to start** if the secret file has incorrect permissions
    - This prevents accidentally exposing the encryption key to other users on the system

- **Backup**: Back up the secret file securely - without it, encrypted passwords cannot be decrypted
- **Storage**: Store the secret file separately from configuration files
- **Never Commit**: Never commit the secret file to version control
- **Rotation**: If the secret file is lost or compromised, you'll need to regenerate it and re-enter all passwords

**Example - Verify Permissions**:
```bash
ls -la pgedge-mcp-server.secret
# Should show: -rw------- (600)

# Fix if needed:
chmod 600 pgedge-mcp-server.secret
```

**Server will exit with an error if permissions are incorrect**:
```
ERROR: Failed to load encryption key from /path/to/pgedge-mcp-server.secret:
insecure permissions on key file: 0644 (expected 0600).
Please run: chmod 600 /path/to/pgedge-mcp-server.secret
```
