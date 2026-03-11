Deploy vectorspace-adserver to production (EC2 via Docker on GHCR).

<!-- RELATED: This command is paired with /rollback. Keep deployment logic in sync. -->
<!-- When updating: check /rollback for health endpoints, verification steps -->

## Quick Deploy

Run these steps in sequence. Stop if any fails.

1. Check git is clean:
```bash
git diff --quiet && git diff --cached --quiet || echo "ERROR: Uncommitted changes"
```

2. Check what changed since last deploy:
```bash
git tag --list 'v*' --sort=-v:refname | head -1
```
Use the tag from above:
```bash
git diff --name-only <TAG>..HEAD
```

Analyze the changed files to determine what's affected:

| Changed Path | Affects |
|-------------|---------|
| `handler/` | Server (API logic) |
| `platform/` | Server (data layer) |
| `cmd/server/` | Server (entrypoint) |
| `portal/` | Portal (frontend, bundled in server image) |
| `sidecar/` | Sidecar (embedding service — separate image) |
| `landing/` | Landing page (S3/CloudFront — needs `make deploy`) |
| `infra/` | Infrastructure (needs `make deploy`) |
| `skill/` | None (runtime data, not deployed) |

If NO changes affect deployable services (e.g., only docs, .claude/, skill/), report "No deployment needed" and stop.

3. Run tests:
```bash
go test ./... -count=1
```

4. Type-check portal (if portal/ changed):
```bash
cd portal && npx tsc --noEmit && cd ..
```

5. Build and push Docker images. Determine which images need building from step 2.

**Server image** (if handler/, platform/, cmd/, portal/, go.mod, Dockerfile changed):
```bash
docker build --platform linux/amd64 --build-arg GIT_HASH=$(git rev-parse --short HEAD) -t ghcr.io/kimjune01/vectorspace-server:$(git rev-parse --short HEAD) -t ghcr.io/kimjune01/vectorspace-server:latest .
docker push ghcr.io/kimjune01/vectorspace-server:$(git rev-parse --short HEAD)
docker push ghcr.io/kimjune01/vectorspace-server:latest
```

**Sidecar image** (if sidecar/ changed):
```bash
docker build --platform linux/amd64 -t ghcr.io/kimjune01/vectorspace-sidecar:$(git rev-parse --short HEAD) -t ghcr.io/kimjune01/vectorspace-sidecar:latest sidecar/
docker push ghcr.io/kimjune01/vectorspace-sidecar:$(git rev-parse --short HEAD)
docker push ghcr.io/kimjune01/vectorspace-sidecar:latest
```

6. Get server IP and SSH via EC2 Instance Connect to pull + restart:
```bash
cd infra && set -a && . ./.env && set +a && pulumi stack output serverIp --stack dev && cd ..
```

Generate a temporary SSH key and push it via EC2 Instance Connect:
```bash
INSTANCE_ID=$(cd infra && pulumi stack output instanceId --stack dev && cd ..)
ssh-keygen -t ed25519 -f /tmp/ec2-connect-key -N "" -q -y 2>/dev/null || ssh-keygen -t ed25519 -f /tmp/ec2-connect-key -N "" -q
aws ec2-instance-connect send-ssh-public-key --instance-id $INSTANCE_ID --instance-os-user ubuntu --ssh-public-key file:///tmp/ec2-connect-key.pub
```

Then SSH within 60 seconds (the pushed key expires):
```bash
ssh -i /tmp/ec2-connect-key -o StrictHostKeyChecking=no ubuntu@<SERVER_IP> "cd /opt/vectorspace && sudo docker compose pull && sudo docker compose up -d"
```

If the server needs GHCR auth to pull private images:
```bash
ssh -i /tmp/ec2-connect-key -o StrictHostKeyChecking=no ubuntu@<SERVER_IP> "echo <GHCR_TOKEN> | sudo docker login ghcr.io -u kimjune01 --password-stdin && cd /opt/vectorspace && sudo docker compose pull && sudo docker compose up -d"
```
Get a GHCR token locally with `gh auth token`.

7. If landing/ or infra/ changed, run Pulumi deploy:
```bash
make deploy
```

8. Verify services are responding and git hash matches:
```bash
curl -sf "https://api.vectorspace.exchange/health" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d)"
```

The `gitHash` field must match the local commit (`git rev-parse --short HEAD`). Retry with exponential backoff (2-32s) if the server hasn't restarted yet.

9. Create and push incremented version tag:
```bash
git tag --list 'v*' --sort=-v:refname | head -1
```
Increment the tag (e.g., if output is `v3`, use `v4`). If no tags exist, start with `v1`:
```bash
git tag <NEW_TAG> && git push origin <NEW_TAG>
```

10. Report success with deployed services, git hash, and the new version tag.

## First-Time Setup

### GHCR Authentication (local machine)
```bash
gh auth token | docker login ghcr.io -u kimjune01 --password-stdin
```
Requires `write:packages` scope. If missing: `gh auth refresh -s write:packages`.

### GHCR Authentication (EC2 server)
The server also needs GHCR auth to pull private images. SSH in and run:
```bash
echo <TOKEN> | sudo docker login ghcr.io -u kimjune01 --password-stdin
```
This persists across pulls until the token expires.

### EC2 SSH Access
Uses EC2 Instance Connect — no key pair needed. The deploy steps above handle key generation and pushing automatically. Requires `aws ec2-instance-connect send-ssh-public-key` permission in your IAM policy.

### Environment Variables
Copy and fill in `infra/.env` from `infra/.env.example`.

## Health Endpoints

| Service | Endpoint |
|---------|----------|
| API | `https://api.vectorspace.exchange/health` |

## Troubleshooting

| Error | Fix |
|-------|-----|
| Docker push 403 | Re-authenticate: `gh auth token \| docker login ghcr.io -u kimjune01 --password-stdin`. May need `gh auth refresh -s write:packages` |
| SSH connection refused | Security group must allow port 22. Uses EC2 Instance Connect (no key pair needed) |
| Docker pull unauthorized on server | Server needs GHCR login: SSH in and run `echo $(gh auth token) \| sudo docker login ghcr.io -u kimjune01 --password-stdin` |
| Health check fails after deploy | SSH in and check logs: `sudo docker compose -f /opt/vectorspace/docker-compose.yml logs server`. Note: server takes ~20s to seed demo data on first boot |
| EBS volume not at /dev/xvdf | On NVMe instances (t3, c5, etc.) it appears as `/dev/nvme1n1`. Check `lsblk` |
| Docker not installed on new instance | UserData uses Docker's official apt repo. If it failed, SSH in and install manually: see infra/main.go UserData |
| Pulumi conflict | Run `cd infra && pulumi cancel` then retry |
| Portal shows stale content | Hard refresh (Cmd+Shift+R) — Vite uses content hashing for cache busting |
