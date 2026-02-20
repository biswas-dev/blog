#!/bin/bash
set -e

# Script to configure nginx for staging
DOMAIN="staging.anshumanbiswas.com"
NGINX_CONFIG="/etc/nginx/sites-available/${DOMAIN}"

echo "⚙️  Configuring nginx for ${DOMAIN}..."

# Create nginx configuration
sudo tee "$NGINX_CONFIG" > /dev/null << 'EOF'
server {
    listen 80;
    server_name staging.anshumanbiswas.com;

    # Redirect HTTP to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl;
    server_name staging.anshumanbiswas.com;

    # SSL certificate (managed by Cloudflare Origin Certificate)
    ssl_certificate /etc/nginx/ssl/staging.anshumanbiswas.com.crt;
    ssl_certificate_key /etc/nginx/ssl/staging.anshumanbiswas.com.key;

    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Proxy to blog application
    location / {
        proxy_pass http://127.0.0.1:22222;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
    }
}
EOF

# Remove default site to avoid SSL conflicts
sudo rm -f /etc/nginx/sites-enabled/default

# Enable site if not already enabled
if [ ! -L "/etc/nginx/sites-enabled/${DOMAIN}" ]; then
    echo "🔗 Enabling nginx site..."
    sudo ln -sf "$NGINX_CONFIG" "/etc/nginx/sites-enabled/${DOMAIN}"
fi

# Test nginx configuration
echo "🧪 Testing nginx configuration..."
sudo nginx -t

echo "✅ Nginx configuration updated successfully!"
echo "📁 Config file: ${NGINX_CONFIG}"
echo ""
echo "To apply changes, run: sudo systemctl reload nginx"
