#!/bin/bash
set -euo pipefail

# Log everything to /var/log/user-data.log
exec > >(tee /var/log/user-data.log) 2>&1

echo "=== vectorspace.exchange bootstrap ==="

# Install Docker
apt-get update -y
apt-get install -y docker.io docker-compose-plugin awscli
systemctl enable docker
systemctl start docker

# Format and mount EBS volume for data persistence
if ! blkid /dev/xvdf; then
  mkfs.ext4 /dev/xvdf
fi
mkdir -p /data
mount /dev/xvdf /data
echo '/dev/xvdf /data ext4 defaults,nofail 0 2' >> /etc/fstab

# Create app directory
mkdir -p /opt/vectorspace

# Write Caddyfile (DOMAIN is replaced by Pulumi templating)
cat > /opt/vectorspace/Caddyfile <<'CADDY'
api.__DOMAIN__ {
	reverse_proxy server:8080
}

portal.__DOMAIN__ {
	reverse_proxy server:8080
}
CADDY

# Write docker-compose.yml (variables replaced by Pulumi templating)
cat > /opt/vectorspace/docker-compose.yml <<'COMPOSE'
services:
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    network_mode: host
    volumes:
      - /opt/vectorspace/Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config

  server:
    image: __SERVER_IMAGE__
    restart: unless-stopped
    network_mode: host
    volumes:
      - /data:/data
    command:
      - "-db-path=/data/vectorspace.db"
      - "-seed"
      - "-admin-password=__ADMIN_PASSWORD__"
      - "-anthropic-key=__ANTHROPIC_API_KEY__"
      - "-sidecar-url=http://127.0.0.1:8081"

  sidecar:
    image: __SIDECAR_IMAGE__
    restart: unless-stopped
    network_mode: host

volumes:
  caddy_data:
  caddy_config:
COMPOSE

# Pull images and start services
cd /opt/vectorspace
docker compose pull
docker compose up -d

echo "=== bootstrap complete ==="
