# Design

## Context

The repository contains four containerized services:
- `api` and `worker`: Go services compiled from source into static Linux binaries and deployed on Alpine.
- `frontend`: Next.js 15 standalone application running under Node Alpine.
- `ai-service`: Python FastAPI service utilizing OpenAI, Groq, and Qdrant.

Recent October 2026 advisories mandate patched runtimes to neutralize RCE, DNS spoofing, and certificate validation vulnerabilities.

## Goals / Non-Goals

**Goals:**
- Update language compiler and runtime baselines to patched versions across all Dockerfiles and dependency manifests.
- Upgrade Alpine base images to `alpine:3.22` with security updates.
- Ensure 100% build compatibility with zero changes to business or API logic.

**Non-Goals:**
- Upgrading major application frameworks (e.g. Next.js 15, FastAPI).
- Altering any database schemas or migration scripts.

## Decisions

### 1. Upgrade Node.js Container from 20 to 22 LTS
- **Decision**: Update `frontend.Dockerfile` to `node:22-alpine` across `deps`, `builder`, and `runner` stages. Add `"engines": { "node": ">=22.23.4" }` to `frontend/package.json`.
- **Rationale**: Node 22 LTS incorporates the patched Brotli engine and truncated UDP resolution handling.
- **Alternatives Considered**: Patching Node 20 packages manually via apk; rejected because Node 20 reached end-of-support window for critical CVE backports and Next.js 15 operates natively on Node 22.

### 2. Pin Go Compiler to 1.25.9 and Harden Alpine Base
- **Decision**: Specify `go 1.25.9` in `go.mod`, use `golang:1.25.9-alpine` for builder stages, and update runtime stage from `alpine:3.20` to `alpine:3.22` with `apk --no-cache upgrade`.
- **Rationale**: Eliminates standard library HTTP/2 and crypto/x509 flaws, while picking up musl libc fixes.
- **Alternatives Considered**: Upgrading to a newer major Go toolchain; rejected to avoid potential breaking changes in Go toolchain semantics.

### 3. Upgrade Python Base to 3.12.15-slim and Enforce `certifi>=2026.10.19`
- **Decision**: Update `ai-service.Dockerfile` to `python:3.12.15-slim` and pin `certifi>=2026.10.19` in `requirements.txt`.
- **Rationale**: Ensures TLS certificate verification distrusts revoked/compromised intermediate CAs.
- **Alternatives Considered**: Maintaining Python 3.11 with manual root store injection; rejected due to maintenance overhead.

## Risks / Trade-offs

- **[Risk] Docker build cache invalidation** → **Mitigation**: Base image updates invalidate the first layer. Dependency files (`go.mod`, `package.json`, `requirements.txt`) are copied before application source, maintaining layer caching efficiency.
- **[Risk] Next.js Node 22 compatibility** → **Mitigation**: Next.js 15 fully supports Node 20 and Node 22. Build verification step checks `next build` execution.
