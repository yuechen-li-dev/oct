# P10 M1 — Prometheus Dominatus Subsystem Design

## 1) Why P10 exists

Prometheus SGEMM has converged on a true reactor architecture (judgment engine, policy memory, slot HFSM, async lifecycle, transfer queue handling, buffering selector, and diagnostics-rich observability). In current code, those concerns are represented in large runtime structs that already function like an implicit blackboard.

P10 exists to make that blackboard explicit **before** adding higher-concurrency behavior (N-slot, work-stealing, multi-stage producers/consumers). The core requirement is preserving non-tearing reads and explicit promotion boundaries between staged writes and decision-time reads.

This milestone (M1) is design only: it defines a Prometheus-local Dominatus subsystem for SGEMM now and FFT later, without introducing a generic runtime kernel or changing Vulkan behavior.

---

## 2) Current implicit blackboard audit

### Audited components

The current implicit blackboard is spread across:

- `internal/prometheus/native/reactor_vulkan.c`
- `internal/prometheus/native/reactor_judgment_engine.h/.c`
- `internal/prometheus/native/reactor_policy_memory.h/.c`
- `internal/prometheus/native/reactor_slot_hfsm.h/.c`
- `internal/prometheus/native/reactor_api.h` (diagnostics export surface)

### Giant implicit blackboard struct (primary)

`prometheus_runtime` in `reactor_vulkan.c` is the primary implicit blackboard owner. It includes mixed concerns in one aggregate:

- Vulkan object handles and queue topology facts.
- SGEMM buffer shape/capability/memory-path facts.
- async lifecycle facts (`async_state`, `async_task_id`, selected path, stage/detail).
- policy + mode-selection state via embedded `prom_sgemm_controller_state`.
- slot ownership/lifecycle state via `slots[2]` and `slot_diag`.

### Secondary giant blackboard slices

1. `prom_sgemm_controller_state` (embedded in `prometheus_runtime`)
   - policy memory/facts/thresholds
   - mode and shape memory (`last_shape_*`, `last_mode`)
   - counters (retreat/recovery/instability/etc.)
   - packed4 and fp16 selection/fallback diagnostics

2. `prom_slot_runtime_diag` (embedded in `prometheus_runtime`)
   - current/next slot ownership IDs
   - slot invalidation/rejection/failure counters
   - transfer queue topology/policy and fallback details
   - m35 buffering feasibility/score/headroom/reason counters
   - proxy-unit progression counters (pull-lag and serial behavior)

3. `PrometheusSgemmPolicyDiagnostics` (API export struct)
   - externalized copy/projection of controller + slot/transfer/m35 diagnostics

### Field categories currently acting as blackboard facts

1. **Policy facts**
   - `prom_policy_memory`, `prom_policy_facts`, `prom_policy_thresholds`
   - `current_mode`, waste ratios, cooldowns, overrides

2. **Selected modes/candidates**
   - path/compute decisions from `prom_judgment_decision`
   - buffering mode decisions (`prom_buffering_selector_decision`)
   - packed4/fp16 selection + reject reasons

3. **Slot lifecycle state**
   - slot HFSM state stack/diagnostics/metadata
   - runtime current/next slot IDs, failure slot/reasons, async slot ownership

4. **Queue topology facts**
   - queue family IDs, dedicated transfer availability, queue-family divergence
   - transfer policy selected, transfer fallback reason, handoff/wait counters

5. **Layout/precision facts**
   - per-slot layout metadata; packed4 row-major/tail checks
   - fp16 tolerance status and rejection details

6. **Memory feasibility facts**
   - required capacity metadata per slot
   - mode headroom fields and budget rejection counters
   - direct/staged/device-local/host-visible capabilities

7. **Counters/reasons/diagnostics**
   - policy and mode transition counters
   - invalidation, rejection, cleanup, starvation, and failure counters
   - explicit reason/detail codes

8. **Async readiness state**
   - lifecycle/state/stage/detail and readiness/consumption semantics
   - transfer completion synchronization and in-flight ownership checks

### Architectural pain in current shape

- Decision-time facts and mutating runtime facts are co-resident in large structs.
- No explicit staged vs visible split; reads can observe partially-updated mixed concerns as concurrency grows.
- Dirty-change semantics are implicit (counter deltas), not typed by keys/domains.
- Ownership transitions exist, but as ad hoc field mutations instead of staged events.

---

## 3) Dominatus explanation for Prometheus

In this Prometheus-local context, **Dominatus** means:

- **Blackboard state**: typed facts/counters/reasons for reactor domains.
- **Dirty keys**: typed markers of which facts changed this step.
- **Staged writes**: reactor/HFSM/completion handlers write to staged buffers.
- **Visible snapshot**: judgment/policy/diagnostics read from a stable visible view.
- **Promotion boundary**: explicit commit step promotes staged -> visible.
- **Ownership events**: lifecycle and handoff transitions recorded as staged events.
- **Actuators/effectors**: subsystems that act on decisions (submission, slot transition, cleanup).
- **Diagnostics/trace**: inspectable history of what changed, by whom, and why.

This is **not** full Dominatus runtime scope (no persistence, no mailbox actor framework, no generic kernel). It is a Prometheus-local reactor substrate.

---

## 4) Proposed blackboard domains

### 4.1 SGEMM domain

Holds shape/layout/precision/selection facts:

- shape `(m, n, k)` and work-units signature
- layout/precision and path/compute decisions
- selected buffering mode
- candidate scores and winner metadata
- packed4/fp16 feasibility, selection, and reject reasons
- fallback reasons (path, transfer, precision)

### 4.2 Slot domain

Holds lifecycle ownership facts:

- slot HFSM state by slot
- slot generation/validity
- current/next/async owner slot IDs
- in-flight flags
- failure slot + reason

### 4.3 Queue domain

Holds topology/transfer policy facts:

- compute queue family index
- transfer queue family index
- dedicated transfer availability
- policy flag for dedicated transfer use
- queue-family-handoff counters
- transfer completion/wait counters

### 4.4 Memory domain

Holds feasibility and capacity facts:

- required capacities per artifact (A/B/C and slots)
- available budget/capability summary
- per-buffering-mode headroom facts
- capacity invalidation flags

### 4.5 Diagnostics domain

Holds reactor-level observability:

- monotonic counters
- reason/detail codes
- last transition and last event
- dirty-key masks and generation metadata

### 4.6 Future FFT domain (reserved)

Additive domain model:

- FFT-specific problem facts (shape/radix/layout)
- FFT mode candidates and rejection reasons
- FFT memory/batching feasibility facts

No SGEMM-domain rewrite required if keys remain domain-scoped.

---

## 5) Proposed typed key schema

Use C enum keys (stable integral IDs), grouped by domain.

```c
typedef enum prom_dom_domain {
  PROM_DOM_SGEMM = 1,
  PROM_DOM_SLOT = 2,
  PROM_DOM_QUEUE = 3,
  PROM_DOM_MEMORY = 4,
  PROM_DOM_DIAG = 5,
  PROM_DOM_FFT = 6,
} prom_dom_domain;

typedef enum prom_dom_key {
  /* SGEMM */
  PROM_DOM_KEY_SGEMM_SHAPE = 0x0101,
  PROM_DOM_KEY_SGEMM_LAYOUT = 0x0102,
  PROM_DOM_KEY_SGEMM_PRECISION = 0x0103,
  PROM_DOM_KEY_SGEMM_PATH_MODE = 0x0104,
  PROM_DOM_KEY_SGEMM_COMPUTE_MODE = 0x0105,
  PROM_DOM_KEY_SGEMM_BUFFERING_MODE = 0x0106,
  PROM_DOM_KEY_SGEMM_FALLBACK_REASON = 0x0107,

  /* SLOT */
  PROM_DOM_KEY_SLOT_STATE = 0x0201,
  PROM_DOM_KEY_SLOT_GENERATION = 0x0202,
  PROM_DOM_KEY_SLOT_VALID = 0x0203,
  PROM_DOM_KEY_SLOT_CURRENT_ID = 0x0204,
  PROM_DOM_KEY_SLOT_NEXT_ID = 0x0205,
  PROM_DOM_KEY_SLOT_FAILURE_REASON = 0x0206,

  /* QUEUE */
  PROM_DOM_KEY_QUEUE_COMPUTE_FAMILY = 0x0301,
  PROM_DOM_KEY_QUEUE_TRANSFER_FAMILY = 0x0302,
  PROM_DOM_KEY_QUEUE_DEDICATED_AVAILABLE = 0x0303,
  PROM_DOM_KEY_QUEUE_TRANSFER_POLICY = 0x0304,
  PROM_DOM_KEY_QUEUE_HANDOFF_COUNT = 0x0305,

  /* MEMORY */
  PROM_DOM_KEY_MEMORY_REQUIRED_CAPACITY = 0x0401,
  PROM_DOM_KEY_MEMORY_BUDGET = 0x0402,
  PROM_DOM_KEY_MEMORY_HEADROOM = 0x0403,
  PROM_DOM_KEY_MEMORY_INVALIDATION_FLAGS = 0x0404,

  /* DIAGNOSTICS */
  PROM_DOM_KEY_DIAG_REASON_CODE = 0x0501,
  PROM_DOM_KEY_DIAG_COUNTER = 0x0502,
  PROM_DOM_KEY_DIAG_LAST_TRANSITION = 0x0503,

  /* FFT reserved */
  PROM_DOM_KEY_FFT_PLAN_SHAPE = 0x0601,
} prom_dom_key;
```

### Key schema requirements satisfied

- typed, stable integer IDs
- no string-key map
- domain separability via encoded ranges and/or domain enum
- additive FFT keyspace reserved without SGEMM churn

---

## 6) Staged / visible state model

### 6.1 Storage model

`prom_blackboard_state` has two instances:

- `visible`: read-only during a decision step
- `staged`: mutable during execution step

### 6.2 Access model

- **Judgment engine / policy selector / diagnostics export** read `visible` snapshot only.
- **Reactor execution / slot HFSM bridge / completion handlers / failure paths** write `staged`.

### 6.3 Commit boundary

At end of a reactor step (or explicit boundary):

```text
staged state -> visible state
```

Commit operation responsibilities:

1. apply staged key updates atomically (per blackboard generation)
2. publish staged ownership events into visible event window
3. increment generation counter
4. clear staged dirty masks/event buffer for next step

### 6.4 Required invariant

- Judgment reads visible only.
- Reactor writes staged only.
- Commit is the only promotion path.

This is the anti-tearing substrate for future concurrency.

---

## 7) Dirty-key tracking design

### 7.1 Mechanism

Track dirty changes with fixed-size bitsets:

- `dirty_keys_staged[KEY_WORDS]`
- `dirty_domains_staged[DOMAIN_WORDS]`
- `dirty_slots_mask_staged` (bit-per-slot)
- `staged_generation` and `visible_generation`

Setters mark key + domain + slot (if relevant) when value changes.

### 7.2 Commit semantics

- Commit copies changed keys into visible state.
- Visible side records `last_commit_dirty_keys` and `last_commit_dirty_domains` for diagnostics.
- Staged dirty masks are cleared post-commit.

### 7.3 Required use cases

1. **Buffer invalidation**
   - A invalidation depends on `(m,k,layout,precision,capacity)` keys
   - B invalidation depends on `(k,n,layout,precision,capacity)` keys
   - C invalidation depends on `(m,n,layout,precision,capacity)` keys
   - evaluator checks only relevant dirty keys before re-alloc/invalidate

2. **Judgment fact stability**
   - if key dependency set unchanged, skip recomputing derived judgment facts
   - preserves deterministic decisions and reduces churn

3. **Slot readiness tracking**
   - slot dirty mask reveals which slot states changed since last boundary
   - enables cheap “changed slots only” transition and readiness checks

4. **Diagnostics**
   - export key/domain delta mask alongside current values
   - clients can inspect “what changed” vs only final state

---

## 8) Ownership / event staging design

### 8.1 Event model

Add a lightweight staged event ring (fixed capacity per step):

```c
typedef enum prom_dom_event_kind {
  PROM_DOM_EVT_SLOT_PREPARED = 1,
  PROM_DOM_EVT_SLOT_READY = 2,
  PROM_DOM_EVT_SLOT_SUBMITTED = 3,
  PROM_DOM_EVT_SLOT_COMPLETE = 4,
  PROM_DOM_EVT_SLOT_FAILED = 5,
  PROM_DOM_EVT_TRANSFER_COMPLETE = 6,
  PROM_DOM_EVT_QUEUE_HANDOFF = 7,
  PROM_DOM_EVT_POLICY_SELECTED_MODE = 8,
  PROM_DOM_EVT_FALLBACK_EMITTED = 9,
} prom_dom_event_kind;
```

Each event contains: kind, slot_id (optional), reason/detail code, generation-local sequence, and source subsystem.

### 8.2 Lifecycle semantics

- events are written to staged event buffer
- events are promoted on commit with state changes
- visible event window becomes diagnostics/inspection source

No mailbox/actor dispatch is introduced in M1.

---

## 9) Traceability / debugging seam

Define a compact runtime trace ring (`N` entries, overwrite oldest) with fields:

- generation/tick
- source subsystem (`judgment`, `slot_hfsm`, `queue`, `memory`, `async`, etc.)
- key changed (if key mutation)
- old/new scalar summary where cheap
- event kind (if event emission)
- reason/detail code
- slot id if relevant

Purpose is runtime observability for audits, Codex/Claude debugging, Marionette validation, and GPU validation prep. No persistence or replay engine is required.

---

## 10) Relationship to current systems

### 10.1 Judgment engine

- consumes `visible` snapshot or a stable projection struct derived from visible keys
- does not read staged mutations in same step

### 10.2 Policy memory

- remains bounded helper for mode transitions
- its externally meaningful outputs are written via blackboard staged setters + dirty keys

### 10.3 Slot HFSM

- remains legal-transition authority
- transition bridge emits staged ownership events and slot dirty updates

### 10.4 Reactor Vulkan

- long-term shifts from “direct owner of every field” to blackboard adapters/setters
- Vulkan behavior does not change in M1; only architecture contract is defined

### 10.5 Diagnostics API

- target state: export from visible snapshot + last-commit dirty/event views
- avoids ad hoc direct reads from mixed mutable runtime internals

---

## 11) Migration plan

### P10 M1 (this milestone)

Architecture specification only.

### P10 M2 — Blackboard core

Implement:

- visible/staged storage
- typed setters/getters
- dirty masks (keys/domains/slots)
- generation counters
- commit + clear semantics
- trace ring primitives

### P10 M3 — SGEMM diagnostics/facts adapter

- route current diagnostics writes through blackboard setters
- preserve external API behavior
- add dirty-key diagnostics export

### P10 M4 — Slot HFSM event bridge

- slot transitions emit staged ownership events
- dirty slot tracking and failure/cleanup tracing

### P10 M5 — Judgment snapshot integration

- judgment reads stable visible snapshot/projection
- staged writes during step do not influence in-step decisions
- next step reads committed state

### Future

N-slot/work-stealing concurrency only after blackboard substrate is stable and validated.

---

## 12) Non-goals (explicit)

P10 M1 does **not** implement:

- N-slot work stealing
- generic reusable Dominatus runtime
- persistence or replay storage
- mailbox/actor subsystem
- callback/plugin event scripting layer
- FFT implementation
- Vulkan behavior changes

---

## 13) Open questions / risks

1. **Key granularity drift**
   - risk: too-coarse keys force excess recompute; too-fine keys create maintenance burden.

2. **Projection compatibility with existing API**
   - risk: visible snapshot layout may diverge from current `PrometheusSgemmPolicyDiagnostics`; adapter must preserve ABI expectations.

3. **Commit timing boundaries**
   - risk: ambiguous boundaries around async poll/consume paths can blur staged vs visible semantics unless codified per reactor step.

4. **Memory overhead constraints**
   - risk: trace/event rings and dual-state storage add footprint; fixed capacities must be tuned.

5. **Legacy direct-field writes during migration**
   - risk: mixed direct writes + setter writes can break dirty correctness; migration phases must isolate ownership per domain.

6. **Current documentation gap**
   - This Dominatus reactor blackboard model is not yet represented as a first-class native subsystem document elsewhere in `internal/prometheus`; this report establishes that architecture seam for subsequent milestones.

---

## Dominatus invariant for Prometheus

> Reactor writes staged state. Judgment reads visible state. Commit promotes staged to visible. Dirty keys identify what changed. Trace captures why.
