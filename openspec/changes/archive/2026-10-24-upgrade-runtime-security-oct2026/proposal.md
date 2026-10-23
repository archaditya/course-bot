# Proposal

## Why

On 21–22 October 2026, critical security advisories were published affecting Node.js runtimes (Brotli parser out-of-bounds write CVE-2026-111497 and DNS truncation buffer mishandling CVE-2026-105482), Go compiler standard libraries (HTTP/2 stream DoS and crypto/x509 chain verification flaws), and TLS root stores / compromised intermediate CAs in Python. Course-Bot services currently rely on unpatched base images (Node 20, unpinned Go 1.25.0, unhardened Alpine, and Python without pinned certifi). Upgrading these runtimes and toolchains eliminates exposure to known exploits.

## What Changes

- Upgrade Go toolchain requirement to `1.25.9` in `go.mod` and pin builder stages in `build/docker/api.Dockerfile` and `build/docker/worker.Dockerfile` to `golang:1.25.9-alpine`.
- Harden runtime Alpine images in `api.Dockerfile` and `worker.Dockerfile` to `alpine:3.22` with system package upgrades.
- Upgrade frontend container runtime from `node:20-alpine` to `node:22-alpine` across all build stages in `build/docker/frontend.Dockerfile`.
- Enforce Node engine constraint `>=22.23.4` and bump `@types/node` to `^22.7.5` in `frontend/package.json`.
- Update `build/docker/ai-service.Dockerfile` to `python:3.12.15-slim` and pin `certifi>=2026.10.19` in `apps/ai-service/requirements.txt`.

## Capabilities

### New Capabilities
- `runtime-security`: Enforces minimum secure compiler, language, and container runtime baselines across Course-Bot services to mitigate published CVE vulnerabilities.

### Modified Capabilities

## Impact

- **Affected Systems**: `api`, `worker`, `frontend`, and `ai-service` container definitions and manifest configurations.
- **Dependencies**: Go toolchain bumped to 1.25.9, Node engine enforced at >=22.23.4, `@types/node` updated, and `certifi>=2026.10.19` added to Python requirements.
- **Breaking Changes**: None. APIs and internal application code remain unchanged.
