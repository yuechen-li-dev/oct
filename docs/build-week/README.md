# Prometheus Build Week submission packet

**Audited submission head:** 3e41eb67e40445754dfc9e057cc95e4efb471c07
**Audit:** 2026-07-21 12:10:04 Pacific (-07:00)
**Status:** DVT2-M6A closed; M6B not started.

## The five-minute story

During Build Week, Prometheus evolved from an experimental Vulkan runtime with prototype SGEMM shaders into a compiled transformer execution system. It ran the complete Z-Image-Turbo transformer—two noise refiners, two context refiners, and 30 main layers—inside all nine evaluations of one deterministic 512x512 image generation on an 8 GiB NVIDIA RTX 3070.

The official BF16 checkpoint contains 453 tensors and 12,309,817,472 payload bytes, exceeding the GPU’s VRAM. Prometheus keeps immutable weights in system memory, retains FP32 activations on the GPU, and streams bounded manifest-authorized packages. Model-owned GPU memory is 1,005,407,748 bytes with Prefetch, or 643,587,076 bytes in MinimumMemory. The production Auto run generated one deterministic 512x512 image in 165.051 seconds, including all nine model evaluations.

## Packet map

- [Submission copy](SUBMISSION.md)
- [Slide source and speaker notes](SLIDE_BRIEF.md)
- [Presenter briefing](PRESENTER_BRIEF.md)
- [Judge Q&A](JUDGE_QA.md)
- [Claims-to-evidence matrix](CLAIMS_EVIDENCE.md)
- [Eligibility and commit ledger](BUILD_WEEK_SCOPE.md)
- [Current reviewer handoff](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_REVIEWER_HANDOFF.md)

The canonical image is authoritative local evidence, not tracked media: C:\Users\yuech\AppData\Local\oct\evt2-z-image-turbo\shipping_smoke\zimage_turbo_prometheus_seed42.png. Recorded SHA-256: 7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613.
