Rollback to a previous production deployment by tag.

<!-- RELATED: This command is paired with /deploy. Keep deployment logic in sync. -->
<!-- When updating: check /deploy for health endpoints, image names, verification steps -->

## Usage
- `/rollback` - Rollback to previous version (one before current)
- `/rollback <tag>` - Rollback to specific tag (e.g., `/rollback v3`)

## Steps

1. Get current production git hash and fetch tags:
```bash
curl -sf "https://api.vectorspace.exchange/health" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('gitHash','unknown'))"
```
```bash
git fetch --tags
```
Compare the production hash against tags to find which tag is currently deployed.

2. List recent tags to find what's deployed:
```bash
git tag --list 'v*' --sort=-v:refname | head -5
```

3. Determine target tag:
- If no argument: use the tag before the currently running tag
- If argument provided: use that tag

4. Check if already running target:
- Compare the target tag's hash (`git rev-parse --short <target_tag>`) against the current production hash
- If they match, report "Production is already running tag X" and stop

5. Show rollback info and confirm:
```bash
git log -1 --format='%h %s' <target_tag>
```
Ask user to confirm: "Rollback from tag X to tag Y? (y/n)"
- Proceed only if user confirms
- If declined, report "Rollback cancelled" and stop

6. Checkout target tag:
```bash
git checkout <target_tag>
```

7. Rebuild and push server image from that tag:
```bash
docker build --platform linux/amd64 --build-arg GIT_HASH=$(git rev-parse --short HEAD) -t ghcr.io/kimjune01/vectorspace-server:latest .
docker push ghcr.io/kimjune01/vectorspace-server:latest
```

8. Get server IP and SSH to pull + restart:
```bash
cd infra && set -a && . ./.env && set +a && pulumi stack output serverIp --stack dev && cd ..
```
```bash
ssh ubuntu@<SERVER_IP> "cd /opt/vectorspace && sudo docker compose pull && sudo docker compose up -d"
```

9. Verify health endpoint and git hash matches target:
```bash
curl -sf "https://api.vectorspace.exchange/health" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d)"
```

The `gitHash` field must match the target tag's commit hash. Retry with exponential backoff (2-32s) if the server hasn't restarted yet.

10. Return to main:
```bash
git checkout main
```

11. Report success with rolled-back tag and git hash.

## Troubleshooting

If rollback fails mid-deploy, you're in detached HEAD state. Run `git checkout main` to return.

## Health Endpoints

| Service | Endpoint |
|---------|----------|
| API | `https://api.vectorspace.exchange/health` |
