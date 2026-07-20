# PROMETHEUS DVT-2 M0 — Persistent Warm Session (Approved Rescope)

## Outcome

M0 retains one Python-visible native session across all nine scheduler evaluations and now retains every lock-validated immutable layer package in host memory for that session. The GPU remains single-window: the accepted 643,587,076-byte model-owned ceiling is unchanged.

This is an honest rescope. The current native reactor still has one active prom_model_block_state, so its family-specific owner is reconstructed while moving NoiseRefiner → ContextRefiner → MainTransformer. M1 is the bounded shared-owner-retargeting change; M0 does not claim that those owner allocations already persist.

## Measured result

The canonical fixed smoke completed all nine evaluations in 209.8794111 s, below the Pre-M0 warm 267.3846363 s (a 57.5052252 s, 21.51% reduction). Every evaluation reported 30 MainTransformer layers, every immutable-payload read probe was zero, and the output PNG SHA-256 remained 7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613.

The session caches 12,286,112,768 host bytes. Peak sampled Python RSS consequently rose to 19,054,112,768 bytes while the cache was live, then fell after deterministic session close. This is host RAM, not model-owned GPU allocation.

## M1 handoff

Implement a closed shared active-owner retarget operation over the existing single GPU arena. It must preserve the current host package store and upload/rebind semantics, while retaining pipelines, descriptor infrastructure, and reusable arenas across family changes. Measured remaining parameter rebind time is approximately 1.35–1.45 s per evaluation. No prefetch, second weight window, typed transport, or scheduler migration is authorized.
