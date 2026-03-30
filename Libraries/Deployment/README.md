# Deployment

## Purpose

`Deployment` provides explicit deployment helpers for a Gitea-backed Oct lab.

## Deployment.Gitea M0 scope

M0 intentionally stays narrow and explicit:

- config generation (`CreateGiteaInstanceConfig`)
- config validation (`ValidateGiteaInstanceConfig`)
- `app.ini` text generation (`GiteaAppIniText`)
- directory layout planning (`OctLabDirectoryPlan`)
- explicit clone command planning (`OctLabCloneCommand`)

## Non-goals

This package is **not**:

- a full Gitea API wrapper
- a complete deployment/orchestration framework
- a hidden one-click installer
- a service lifecycle manager

## Architectural note

`Deployment.Gitea` is designed to make setup explicit and repeatable by producing deterministic deployment inputs (config, text, directory plans, and commands) rather than hidden automation.
