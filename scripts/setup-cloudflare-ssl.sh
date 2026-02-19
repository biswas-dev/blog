#!/bin/bash
set -e

# Script to automatically generate and install Cloudflare Origin Certificate
# Usage: ./setup-cloudflare-ssl.sh <domain> <cloudflare_api_token>

DOMAIN="${1:-staging.anshumanbiswas.com}"
CF_API_TOKEN="${2:-$CF_API_TOKEN}"
CERT_DIR="/etc/nginx/ssl"
CERT_FILE="${CERT_DIR}/${DOMAIN}.crt"
KEY_FILE="${CERT_DIR}/${DOMAIN}.key"

# Check for required tools
if ! command -v jq &> /dev/null; then
    echo "📦 Installing jq..."
    sudo apt-get update -qq && sudo apt-get install -y -qq jq
fi

if [ -z "$CF_API_TOKEN" ]; then
    echo "❌ Error: Cloudflare API token not provided"
    echo "Usage: $0 <domain> <cloudflare_api_token>"
    echo "Or set CF_API_TOKEN environment variable"
    exit 1
fi

echo "🔐 Setting up Cloudflare Origin Certificate for ${DOMAIN}"

# Check if certificate already exists and is valid for more than 30 days
if [ -f "$CERT_FILE" ]; then
    echo "📋 Checking existing certificate..."
    EXPIRY_DATE=$(openssl x509 -enddate -noout -in "$CERT_FILE" | cut -d= -f2)
    EXPIRY_EPOCH=$(date -d "$EXPIRY_DATE" +%s 2>/dev/null || date -j -f "%b %d %T %Y %Z" "$EXPIRY_DATE" +%s)
    CURRENT_EPOCH=$(date +%s)
    DAYS_LEFT=$(( ($EXPIRY_EPOCH - $CURRENT_EPOCH) / 86400 ))

    if [ $DAYS_LEFT -gt 30 ]; then
        echo "✅ Certificate is valid for $DAYS_LEFT more days. No renewal needed."
        exit 0
    else
        echo "⚠️  Certificate expires in $DAYS_LEFT days. Generating new certificate..."
    fi
else
    echo "📝 No existing certificate found. Generating new certificate..."
fi

# Create SSL directory if it doesn't exist
sudo mkdir -p "$CERT_DIR"

# Generate private key
echo "🔑 Generating private key..."
openssl genrsa -out /tmp/${DOMAIN}.key 2048

# Generate CSR
echo "📄 Generating CSR..."
openssl req -new -key /tmp/${DOMAIN}.key -out /tmp/${DOMAIN}.csr -subj "/CN=${DOMAIN}"

# Read CSR content
CSR_CONTENT=$(cat /tmp/${DOMAIN}.csr | sed ':a;N;$!ba;s/\n/\\n/g')

# Get Zone ID from domain
ZONE_NAME=$(echo "$DOMAIN" | awk -F. '{print $(NF-1)"."$NF}')
echo "🌐 Looking up Zone ID for ${ZONE_NAME}..."

ZONE_ID=$(curl -s -X GET "https://api.cloudflare.com/client/v4/zones?name=${ZONE_NAME}" \
    -H "Authorization: Bearer ${CF_API_TOKEN}" \
    -H "Content-Type: application/json" | jq -r '.result[0].id')

if [ -z "$ZONE_ID" ] || [ "$ZONE_ID" = "null" ]; then
    echo "❌ Error: Could not find Zone ID for ${ZONE_NAME}"
    echo "Make sure the API token has Zone:Read permission"
    exit 1
fi

echo "✅ Zone ID: ${ZONE_ID}"

# Create Cloudflare Origin Certificate (15 year validity)
echo "🎫 Generating Cloudflare Origin Certificate..."
RESPONSE=$(curl -s -X POST "https://api.cloudflare.com/client/v4/certificates" \
    -H "Authorization: Bearer ${CF_API_TOKEN}" \
    -H "Content-Type: application/json" \
    --data "{
        \"hostnames\": [\"${DOMAIN}\"],
        \"requested_validity\": 5475,
        \"request_type\": \"origin-rsa\",
        \"csr\": \"${CSR_CONTENT}\"
    }")

# Check if request was successful
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
if [ "$SUCCESS" != "true" ]; then
    echo "❌ Error: Failed to generate Cloudflare Origin Certificate"
    echo "$RESPONSE" | jq -r '.errors[]'
    exit 1
fi

# Extract certificate
CERTIFICATE=$(echo "$RESPONSE" | jq -r '.result.certificate')

if [ -z "$CERTIFICATE" ] || [ "$CERTIFICATE" = "null" ]; then
    echo "❌ Error: Certificate not found in response"
    exit 1
fi

# Save certificate and key
echo "💾 Installing certificate..."
echo "$CERTIFICATE" | sudo tee "$CERT_FILE" > /dev/null
sudo cp /tmp/${DOMAIN}.key "$KEY_FILE"

# Set proper permissions
sudo chmod 644 "$CERT_FILE"
sudo chmod 600 "$KEY_FILE"

# Clean up temporary files
rm -f /tmp/${DOMAIN}.key /tmp/${DOMAIN}.csr

# Get certificate expiry date
EXPIRY=$(openssl x509 -enddate -noout -in "$CERT_FILE" | cut -d= -f2)

echo "✅ Cloudflare Origin Certificate installed successfully!"
echo "📅 Certificate valid until: ${EXPIRY}"
echo "📁 Certificate: ${CERT_FILE}"
echo "🔑 Private key: ${KEY_FILE}"
echo ""
echo "Next steps:"
echo "1. Reload nginx: sudo systemctl reload nginx"
echo "2. Verify Cloudflare SSL/TLS mode is set to 'Full (strict)'"
