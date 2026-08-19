---
title: Home
---

# IdMux Proxy

IdMux Proxy is a small Go reverse proxy. It lets one browser use more than
one active session with the same identity provider (IdP).

The IdP stays unchanged. IdMux stores the selected IdP sessions in one
encrypted browser cookie and sends only the selected session upstream.

## Start here

- [Getting started](getting-started.html) — run IdMux on your machine.
- [Configuration](configuration.html) — set the environment variables.
- [Architecture](architecture.html) — see how the proxy works.
- [Operations](operations.html) — run it safely.
- [Security rules](security-invariants.html) — rules that must not change.
- [Development](development.html) — run tests and checks.
- [GitOps](gitops.html) — change, release, and deploy from Git.
- [Contributing](contributing.html) — send a small and useful change.
- [Edit this site](editing.html) — edit the pages on the web or with `gh`.

## Project status

The secure core is ready for local testing and adapter work. Test IdMux with
the exact IdP version and login flow used by your team before production use.

This site is built from the Markdown files in the `docs/` folder. Every page
can be edited on GitHub or from the command line.
