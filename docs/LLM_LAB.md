# LLM Lab Notes

This document is a short orientation for AI-assisted and human-assisted repo navigation.

## Core Boundary

- **Go defines Oct implementation behavior** (`cmd/`, `internal/`, runtime/backend code).
- **Oct defines user-facing contracts and ecosystem behavior** (`Language/`, `Libraries/`).

Do not blur this boundary by embedding broad semantic definitions in Go tests or implementation comments when those semantics already belong in `Language/`.

## Where to Put Things

- New language contract/spec behavior → `Language/`
- New reusable Oct package logic → `Libraries/`
- Backend/runtime/compiler behavior → Go implementation areas
- Backend exploration scaffold/docs → `Backends/ClrOct/`
- Curated teachable examples → `Examples/`
- Exploratory probes and milestone lab work → `Experiments/`

## Documentation Rule

Use files in `docs/` as current truth.

Treat milestone reports as historical context snapshots.
