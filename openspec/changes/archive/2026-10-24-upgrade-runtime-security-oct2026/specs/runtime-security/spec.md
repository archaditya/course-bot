# Spec Delta

## Purpose

Enforces minimum secure compiler, language, and container runtime baselines across Course-Bot services to prevent execution on unpatched environments vulnerable to critical published CVEs.

## ADDED Requirements

### Requirement: Minimum Secure Node.js Runtime
The frontend application and container runtime SHALL enforce a minimum Node.js runtime version of 22.23.4 (or subsequent secure LTS releases) to mitigate Brotli decoder out-of-bounds write vulnerabilities (CVE-2026-111497) and DNS resolution bypasses (CVE-2026-105482).

#### Scenario: Building and running on secure Node.js 22 runtime
- **WHEN** the frontend application container or build script is executed using Node.js v22.23.4 or higher
- **THEN** package installation and Next.js production compilation succeed without engine warnings or Brotli stream errors

#### Scenario: Running on unpatched Node.js 20 runtime
- **WHEN** an environment attempts installation using Node.js version lower than 22.23.4
- **THEN** package manager engine checks flag incompatible runtime versions

### Requirement: Minimum Secure Go Toolchain
The backend API and background worker services SHALL build with a minimum Go version of 1.25.9 to ensure immunity against standard library HTTP/2 stream denial-of-service and crypto/x509 certificate chain validation flaws.

#### Scenario: Compiling backend services with Go 1.25.9
- **WHEN** backend binaries are compiled via `go build`
- **THEN** compilation succeeds against the Go 1.25.9 directive without deprecation errors

#### Scenario: Multi-stage Docker builder image version
- **WHEN** the API or worker Docker container images are built
- **THEN** the builder stage pulls `golang:1.25.9-alpine` and outputs the statically linked executable

### Requirement: Secure Python Runtime and CA Store
The AI service SHALL run on Python 3.12 (specifically `python:3.12.15-slim` or newer) and enforce a minimum `certifi` version of 2026.10.19 to ensure distrust of compromised intermediate CAs and correct TLS verification.

#### Scenario: Python service dependency installation
- **WHEN** pip dependencies are installed in the AI service
- **THEN** the resolver selects `certifi>=2026.10.19` and TLS connections verify against the updated root trust store

#### Scenario: Python container runtime base
- **WHEN** the AI service container image is built
- **THEN** the base image utilizes `python:3.12.15-slim`

### Requirement: Alpine Container OS Hardening
Minimal runtime container images for Go services SHALL use `alpine:3.22` and upgrade OS packages during container image build to remediate upstream OS-level advisories in OpenSSL and musl libc.

#### Scenario: Runtime image package upgrade
- **WHEN** the runtime stage of `api.Dockerfile` or `worker.Dockerfile` builds
- **THEN** packages are upgraded to the latest security patch levels via Alpine repositories
