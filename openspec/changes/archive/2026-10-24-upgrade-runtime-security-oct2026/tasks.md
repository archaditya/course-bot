# Tasks

## 1. Go Toolchain and Alpine Base Hardening

- [x] 1.1 Update `go.mod` to `go 1.25.9` and verify `go build ./...` passes
- [x] 1.2 Update `build/docker/api.Dockerfile` builder to `golang:1.25.9-alpine` and runtime to `alpine:3.22` with package upgrades
- [x] 1.3 Update `build/docker/worker.Dockerfile` builder to `golang:1.25.9-alpine` and runtime to `alpine:3.22` with package upgrades

## 2. Frontend Node.js Runtime Upgrade

- [x] 2.1 Enforce engine constraint `node: ">=22.23.4"` and update `@types/node` in `frontend/package.json`
- [x] 2.2 Update all stages in `build/docker/frontend.Dockerfile` to `node:22-alpine` and verify frontend container build succeeds

## 3. Python AI Service Hardening

- [x] 3.1 Pin `certifi>=2026.10.19` in `apps/ai-service/requirements.txt`
- [x] 3.2 Update `build/docker/ai-service.Dockerfile` base image to `python:3.12.15-slim` and verify container build succeeds

## 4. End-to-End Build & Runtime Verification

- [x] 4.1 Verify `docker-compose.prod.yml` build configurations for all updated services
- [x] 4.2 Verify runtime container reporting for Node.js (`v22.x`), Python (`certifi`), and Go binary health
