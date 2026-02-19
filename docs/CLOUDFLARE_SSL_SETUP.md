# Automated Cloudflare Origin Certificate Setup

This repository includes automated scripts to generate and install Cloudflare Origin Certificates for HTTPS support.

## Why Cloudflare Origin Certificates?

- **Free**: No cost
- **Long-lived**: 15 years validity (no frequent renewal like Let's Encrypt)
- **Automatic**: Generated via API during deployment
- **Secure**: Encrypts traffic between Cloudflare and origin server
- **Flexible**: Works with Cloudflare's "Full (strict)" SSL mode

## Setup Instructions

### 1. Create Cloudflare API Token

1. Go to [Cloudflare Dashboard](https://dash.cloudflare.com/profile/api-tokens)
2. Click **"Create Token"**
3. Use **"Create Custom Token"** template
4. Configure permissions:
   - **Zone → Zone → Read**
   - **Zone → SSL and Certificates → Edit**
5. Set Zone Resources:
   - **Include → Specific zone → anshumanbiswas.com**
6. Click **"Continue to summary"** → **"Create Token"**
7. **Copy the token** (you won't see it again!)

### 2. Add Token to GitHub Secrets

1. Go to your GitHub repository
2. Navigate to **Settings** → **Secrets and variables** → **Actions**
3. Click **"New repository secret"**
4. Name: `CLOUDFLARE_API_TOKEN`
5. Value: Paste the token you copied
6. Click **"Add secret"**

### 3. Update Cloudflare SSL/TLS Mode

1. Go to [Cloudflare Dashboard](https://dash.cloudflare.com)
2. Select your domain **anshumanbiswas.com**
3. Go to **SSL/TLS** → **Overview**
4. Set encryption mode to **"Full (strict)"**
   - This ensures end-to-end encryption with certificate validation

### 4. Deploy

The SSL certificate will be automatically generated and installed during deployment:

```bash
git push origin main  # Triggers staging deployment
```

## How It Works

### Automatic Certificate Generation

The deployment workflow (`deploy-staging.yml`) automatically:

1. **Configures nginx** for HTTPS (port 443)
2. **Generates Cloudflare Origin Certificate** via API if `CLOUDFLARE_API_TOKEN` is available
3. **Installs certificate** to `/etc/nginx/ssl/`
4. **Reloads nginx** to apply changes

### Certificate Lifecycle

- **Validity**: 15 years
- **Renewal**: Not needed for 15 years
- **Automatic check**: Script checks expiry and skips generation if >30 days remain
- **Manual renewal**: Run `./scripts/setup-cloudflare-ssl.sh staging.anshumanbiswas.com` on server

### Scripts

- **`scripts/setup-cloudflare-ssl.sh`**: Generates and installs Cloudflare Origin Certificate
- **`scripts/setup-nginx-staging.sh`**: Configures nginx for HTTPS

## Manual Certificate Generation

If you need to manually generate a certificate:

```bash
# SSH into server
ssh ubuntu@129.213.82.37

# Navigate to deployment directory
cd ~/blog-staging

# Run SSL setup script
sudo CF_API_TOKEN="your_token_here" ./scripts/setup-cloudflare-ssl.sh staging.anshumanbiswas.com

# Reload nginx
sudo systemctl reload nginx
```

## Troubleshooting

### Error: "Could not find Zone ID"

**Cause**: API token doesn't have Zone:Read permission or wrong zone

**Fix**: Recreate API token with Zone:Read permission for anshumanbiswas.com

### Error: "Failed to generate Cloudflare Origin Certificate"

**Cause**: API token doesn't have SSL and Certificates:Edit permission

**Fix**: Recreate API token with correct permissions

### Still seeing "Invalid SSL certificate" (Error 526)

**Causes**:
1. Cloudflare SSL/TLS mode is not set to "Full (strict)" or "Full"
2. Certificate not yet installed on origin
3. Nginx not reloaded after certificate installation

**Fix**:
1. Check Cloudflare SSL/TLS settings
2. Verify certificate exists: `ls -la /etc/nginx/ssl/staging.anshumanbiswas.com.*`
3. Reload nginx: `sudo systemctl reload nginx`

## Security Notes

- **Private keys** are generated on the server and never transmitted
- **API token** should have minimal permissions (Zone:Read + SSL:Edit only)
- **Certificates** are stored in `/etc/nginx/ssl/` with proper permissions (644 for cert, 600 for key)
- **Origin certificates** only work with Cloudflare (not trusted by browsers directly)

## Files

- `.github/workflows/deploy-staging.yml`: Deployment workflow with SSL automation
- `scripts/setup-cloudflare-ssl.sh`: Certificate generation script
- `scripts/setup-nginx-staging.sh`: Nginx configuration script
- `/etc/nginx/ssl/staging.anshumanbiswas.com.crt`: Certificate location (on server)
- `/etc/nginx/ssl/staging.anshumanbiswas.com.key`: Private key location (on server)
