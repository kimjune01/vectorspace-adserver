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

6. Get server IP and SSH to pull + restart:
```bash
cd infra && set -a && . ./.env && set +a && pulumi stack output serverIp --stack dev && cd ..
```
```bash
ssh ubuntu@<SERVER_IP> "cd /opt/vectorspace && sudo docker compose pull && sudo docker compose up -d"
```

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

### GHCR Authentication
```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u kimjune01 --password-stdin
```

### EC2 SSH Access
Set `KEY_NAME` in `infra/.env` to your EC2 key pair name. Ensure the key pair exists in your AWS region.

### Environment Variables
Copy and fill in `infra/.env` from `infra/.env.example`.

## Health Endpoints

| Service | Endpoint |
|---------|----------|
| API | `https://api.vectorspace.exchange/health` |

## Troubleshooting

| Error | Fix |
|-------|-----|
| Docker push 403 | Re-authenticate: `echo $GITHUB_TOKEN \| docker login ghcr.io -u kimjune01 --password-stdin` |
| SSH connection refused | Check KEY_NAME is set in infra/.env and security group allows port 22 |
| Health check fails after deploy | SSH in and check logs: `sudo docker compose -f /opt/vectorspace/docker-compose.yml logs server` |
| Pulumi conflict | Run `cd infra && pulumi cancel` then retry |
| Portal shows stale content | Hard refresh (Cmd+Shift+R) — Vite uses content hashing for cache busting |
