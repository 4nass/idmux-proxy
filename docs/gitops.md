# GitOps workflow

Git is the source of truth for IdMux code, build rules, and deployment inputs.
A change is reviewed, tested, merged, released, and deployed from Git.

## Change flow

1. Create a short-lived branch from `main`.
2. Make one focused change.
3. Run the local checks from `docs/development.md`.
4. Open a pull request with the security impact and test evidence.
5. Wait for required CI, lint, and image security checks.
6. Merge the pull request into `main`.
7. Create a signed release tag such as `v0.1.0`.
8. Deploy the image by digest through the environment configuration.

Do not edit a running environment by hand. Make the desired state change in Git
and let the deployment system apply it.

## Main branch rules

Configure the `main` branch with:

- pull requests required for every change;
- at least one code owner review;
- required status checks: `CI / test`, `Lint / golangci-lint`,
  `Security / govulncheck`, `Security / container-scan`, and
  `CodeQL / analyze`;
- stale approvals dismissed after new changes;
- conversations resolved before merge;
- force pushes and branch deletion disabled;
- dependency and action updates managed by Dependabot.

Keep the branch history linear and use small, focused commits.

## Releases

The release workflow runs only for tags matching `v*.*.*`. It:

- runs the release tests;
- builds a release image and blocks on HIGH or CRITICAL Trivy findings;
- builds Linux AMD64 and ARM64 images;
- publishes the image to GHCR;
- creates SBOM and provenance data.

Deploy a released image by immutable digest:

```text
ghcr.io/4nass/idmux-proxy@sha256:<digest>
```

Do not use `latest` in a production deployment.

## Environments and secrets

Keep environment desired state separate from application source when possible.
Each environment should define:

- the exact IdMux image digest;
- the IdP URL and cookie names;
- the session lifetime and trusted origins;
- resource limits and network policy;
- health and readiness checks.

Never store encryption keys, IdP cookies, tokens, or customer data in Git.
Use GitHub Environments and an external secret manager. Prefer short-lived
OIDC access over long-lived cloud credentials.

## Rollback

Rollback by reverting the environment commit to the last known-good image
digest. Do not rebuild an old source tree with a moving dependency or base image.
Record the incident, affected digest, and validation steps.

## Review checklist

Before merging, confirm:

- the change is small and provider-neutral;
- security invariants are unchanged or documented;
- tests cover new routing and cookie behavior;
- the container still runs as a non-root user;
- no secret or real session value is present;
- the deployment input uses an immutable image digest.
