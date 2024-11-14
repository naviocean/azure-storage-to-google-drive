### Azure Storage Setup

#### Finding Azure Storage Credentials

1. **Via Azure Portal**:
   - Go to [Azure Portal](https://portal.azure.com)
   - Navigate to "Storage accounts"
   - Select your storage account or create new one:
     ```
     Create New Storage Account:
     - Click "+ Create"
     - Fill in basic information:
       * Resource group (create new or select existing)
       * Storage account name (this will be your AZURE_ACCOUNT_NAME)
       * Region
       * Performance: Standard
       * Redundancy: Locally-redundant storage (LRS)
     ```
   - Once created or selected, go to:
     * "Access keys" section under "Security + networking"
     * Click "Show keys"
     * You will find:
       - `AZURE_ACCOUNT_NAME`: Listed as "Storage account name"
       - `AZURE_ACCOUNT_KEY`: Copy "key1" or "key2" value

2. **Create Container**:
   - In your storage account, go to "Containers"
   - Click "+ Container"
   - Name your container (this will be your `AZURE_CONTAINER_NAME`)
   - Set "Public access level" (usually "Private")

3. **Connection String Format**:
   ```
   DefaultEndpointsProtocol=https;AccountName={your-account-name};AccountKey={your-account-key};EndpointSuffix=core.windows.net
   ```

#### Azure Storage Environment Variables

```bash
# Source Storage (for backup)
AZURE_ACCOUNT_NAME=your_storage_account_name       # Example: mystorageaccount
AZURE_ACCOUNT_KEY=your_storage_account_key         # Example: Ab12Cd34Ef56Gh78...
AZURE_CONTAINER_NAME=your_container_name           # Example: backups

# Target Storage (for restore)
TARGET_AZURE_ACCOUNT_NAME=target_storage_account   # Example: restorestorageaccount
TARGET_AZURE_ACCOUNT_KEY=target_account_key        # Example: Xy98Wv76Ut54Rs32...
TARGET_AZURE_CONTAINER_NAME=target_container       # Example: restored
```

#### Azure Storage Access Tips

1. **Permissions Required**:
   - For backup service:
     * Read access to source container
     * List container contents
   - For restore service:
     * Write access to target container
     * Create container if not exists

2. **Security Best Practices**:
   - Use different storage accounts for source and target
   - Rotate access keys periodically
   - Use Managed Identities in production
   - Consider using Azure Key Vault for key storage

3. **Networking**:
   - Check firewall settings if accessing from specific IPs
   - Configure CORS if needed
   - Consider using private endpoints for enhanced security

4. **Monitoring**:
   - Enable Azure Storage metrics
   - Set up alerts for capacity and performance
   - Monitor access patterns

5. **Cost Management**:
   - Choose appropriate redundancy level
   - Set lifecycle management rules
   - Monitor data transfer costs
   - Consider reserved capacity for large deployments

#### Troubleshooting Azure Storage

1. **Common Issues**:
   - "AuthorizationFailure": Check account keys
   - "ContainerNotFound": Verify container name
   - "NetworkError": Check firewall/VNET settings

2. **Validation Steps**:
   ```bash
   # Test Azure Storage connection
   az storage container list \
     --account-name YOUR_ACCOUNT_NAME \
     --account-key YOUR_ACCOUNT_KEY

   # Test container access
   az storage blob list \
     --container-name YOUR_CONTAINER_NAME \
     --account-name YOUR_ACCOUNT_NAME \
     --account-key YOUR_ACCOUNT_KEY
   ```

3. **Performance Tips**:
   - Use closest region for better latency
   - Enable soft delete for recovery
   - Consider premium storage for high-performance needs

4. **Logging**:
   - Enable Storage Analytics logging
   - Check Azure Monitor
   - Review application logs for storage operations