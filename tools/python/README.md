# Python research tools

These scripts are retained because they reproduce or inspect specific Prometheus research artifacts. They are not part of the Oct compiler or ordinary `oct test` workflow.

| Directory | Purpose | Entry points |
| --- | --- | --- |
| [`zimage/`](zimage/) | Z-Image capture, fixed smoke, DVT-2 report materialization, and diagnostic bridges | `zimage_prometheus_smoke.py`, `generate_dvt2_m2_artifacts.py`, `zimage_dvt2_*_report.py` |
| [`gemma/`](gemma/) | Gemma 4 reference oracles for bounded comparisons | `gemma4e2b_reference_oracle.py`, `gemma4e2b_m1_reference.py` |

Run them from the repository root with the pinned local payloads and Python environments described in [EVT-2 local payloads](../../docs/EVT2_LOCAL_PAYLOADS.md) and the relevant development report. The Z-Image bridge and progress modules live beside their callers so imports resolve without installation. The pure progress tests run with `python -m unittest tools/python/zimage/test_zimage_demo_progress.py`.

Avoid adding one-off scripts at the `tools/` root. Give a retained script an owner, reproduction command, input authority, and expected output in its directory or report; leave disposable probes outside the tracked tree.
