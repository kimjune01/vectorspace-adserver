services:
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /opt/vectorspace/Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config

  server:
    image: ${SERVER_IMAGE}
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - /data:/data
    environment:
      - ADMIN_PASSWORD=${ADMIN_PASSWORD}
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    command: ["-db-path=/data/vectorspace.db", "-seed", "-admin-password=${ADMIN_PASSWORD}", "-anthropic-key=${ANTHROPIC_API_KEY}", "-sidecar-url=http://sidecar:8081"]

  sidecar:
    image: ${SIDECAR_IMAGE}
    restart: unless-stopped
    ports:
      - "8081:8081"

volumes:
  caddy_data:
  caddy_config:
