# Prometheus Concept asynchronous prefetch — M11

Status: **MILESTONE VERIFIED** for the bounded Windows AMD64 dual-queue path.

## Ownership and handoff

`PrefetchOwner` in `PrometheusModelBlock.concept` owns a transfer attachment,
one upload staging buffer, pending transfers and an owned queue handoff. The
existing model allocation owner owns both fixed shared weight windows. Prefetch
borrows their immutable extents while compute uses the active window; activation
exchanges binding views after the host task has joined and compute has retired.
No ownership escapes into native packets and no third weight window is allocated.

Concept Vulkan adds `CreateBufferSharedOn<T>` for two queue families. Different
families use concurrent sharing; equal families use exclusive sharing. Native
family bounds are checked before allocation, and the new semaphore record has a
measured ABI claim in the package manifest. Existing ABI record layouts and the
ordinary Submit entry point are preserved.

`SignalHandoff` owns a binary semaphore and producer submission.
`ReceiveHandoff` admits one consumer on the same device and submits a wait at all
commands. `QueueHandoff` field destruction joins consumer, then producer, before
releasing the semaphore. Cancellation with an unconsumed signal is safe too.
This supplies an explicit GPU memory dependency across transfer and compute;
concurrent sharing alone and a CPU fence wait are not used as substitutes.

Prometheus registers the synchronized queue callback through its existing host
queue mutex. Only the brief Vulkan queue call holds that lock; GPU waits do not.
A configured legacy callback cannot be bypassed by a raw semaphore submission.

The handwritten C transfer recorder, acquire recorder, reusable transfer fence
and command-buffer owner are deleted. C retains pinned payload/transition
admission, host byte transport, legacy audit descriptor adaptation and evidence
projection. Both GPU copies and the semaphore handoff execute Concept operations.
The M10 external-window import seam is removed: both windows now belong to Concept.

## Publication, recovery and concurrency

- The entire tensor plan is validated before the first staging write. A malformed
  late tensor cannot partially overwrite a ready window. Duplicate prefetch of
  a ready/acquired window is rejected without destroying its handoff.
- Uploading, Ready, Acquired and Uncertain are explicit Concept phases. A failed
  transfer cannot advance binding/descriptor generations or the retarget position.
  A fresh prefetch retires incomplete work before reusing staging.
- The final transfer signals the handoff. Acquire retries remember whether the
  consumer was already submitted, so completion recovery does not wait a binary
  semaphore twice. Descriptor failure preserves the old family, parameter and
  buffer metadata; a retry can commit the completed candidate once.
- Reset cancels and joins the inactive handoff. Destruction retires prefetch before
  freeing either borrowed window or the device attachment. A session may disappear
  while independent transfer work remains owned by the model.
- Mutable transfer state belongs to a separate Concept object. Prefetch does not
  update shared compute counters, active buffer metadata or common memcpy timing,
  and its response reports operation status rather than sampling mutable compute
  evidence. The public header documents that active evidence sampling, activation,
  reset and destruction require joining the scoped prefetch task.

The transfer API deliberately remains a bounded blocking operation on its scoped
host task, allowing active compute on another host task/queue. One staging buffer
is reused after each observed transfer. This is not a new scheduling framework,
fully nonblocking upload API, or a throughput optimization claim. Native operation
counters/timing remain compute-oriented; transfer and handoff performance telemetry
is not qualified by this milestone.

## Real GPU witness

The existing independent Main witness now includes a fourth scenario requesting
Prefetch. It requires actual profile admission, distinct queues and distinct
families, rather than accepting the minimum-memory fallback. The RTX 3070 selects
compute family **0** and transfer family **1**.

It executes prefetch on a scoped native future concurrently with active Noise0
execution, joins the future, and activates the successor through the Concept
handoff. It then reuses both windows across Noise1, Context0/1 and Main0/1.
All 4,055,040 Main output elements still match independent CPU contractions:
maximum absolute errors 4.06713e-6 and 4.92126e-6 for the first and second layers
in both Normal and Verify. No tolerance or numerical oracle was weakened.

The scenario injects record, submit and completion failures at early and final
transfer tensors; acquire record/submit/completion and descriptor failures;
malformed late tensors; and duplicate ready-window admission. Binding generations
remain unchanged on failure and advance exactly once on activation. It also
resets an unobserved final signal, repairs the first evaluation transition, then
destroys the session/model with another unobserved producer. All Concept buffers
and pipelines return to baseline, and no warm buffer allocation is introduced.
Legacy SGEMM still works on the shared device.

Concurrent host execution on distinct hardware queues is exercised. Actual
simultaneous GPU residency is not established by cross-queue timestamps, and no
transfer/compute overlap ratio or performance improvement is claimed.

## Qualification

Oct base: `c01c30ba`. Concept library dependency: `a585a25`,
[Concept PR #193](https://github.com/yuechen-li-dev/Concept/pull/193).
Target: NVIDIA GeForce RTX 3070, driver 610.88, Vulkan SDK 1.4.350.0,
MSVC 14.51.36231 and Clang 22.1.3. Native runs enable
`PROMETHEUS_VK_VALIDATION=1` and `VK_LAYER_VALIDATE_SYNC=1`.

- The canonical Windows native launcher passes in **161.662 s**, building the
  DLL, normal/slow harnesses and benchmark host. Log:
  `out/test-artifacts/prometheus_native_windows_build.log`.
- **36 Normal native GPU cases pass**, zero skips/failures and no VUID,
  synchronization hazard or validation error. The count is unchanged because the
  new dual-queue scenario extends an existing fact. Logs and counts:
  `out/test-artifacts/prometheus_concept_prefetch_m11/` and `.tmp/m11-final-native.log`.
- **Eight focused Verify model/session/weight cases pass**, including the new
  dual-queue scenario, zero skips/failures/findings. The changed model object
  replaces `006.o` in a copied archive; companions remain Normal. This does not
  claim a Verify rebuild of every dependency. Log: `.tmp/m11-verify-native.log`.
- Three fresh emissions match all six C/header/manifest/map/MIR artifacts. Normal
  and Verify ABI headers match; both emitted C files compile as strict C11 with
  `clang -std=c11 -pedantic-errors`. Products: `.tmp/m11-emission-{1,2,3,verify}/`.
- Concept's Vulkan library suite passes all **18 facts in Normal and Verify**
  against its GPU-free runtime, including canceled signals, duplicate consumers,
  semaphore lifetime and invalid families. Native ABI claims are independently
  measured by that suite. Selected Vulkan profile examples, binding-example
  currency and C ABI boundary tests pass, as do checked outputs and owning-package
  vet. Logs: `.tmp/m11-concept-{final,checked,vet}.log` in the Oct checkout.
- All four explicit research C configurations compile. Generation/deployment
  provenance, native manifest parity, SDSL-V workspace checks, targeted Go tests
  and vet pass. All three SDSL-V module corpus cases pass through the native shader
  host. Logs: `.tmp/m11-experiments-compile.log` and
  `.tmp/m11-check-{generation,manifest,workspace,go,vet,modules}.log`.
- Both repositories pass `git diff --check`. Generated C/headers, JSON/provenance,
  archives, binaries and logs remain ignored. FEEDBACK has 131 unique IDs:
  7 open, 11 deferred, 112 resolved, 1 superseded; next identifier FB-132.

Initial harness compile failures and stale-executable runs are not qualification
for M11. Only the final canonical build and logs above establish the current path.

The original full Z-Image payload/model, Linux/other GPU execution, actual device
loss and race-sanitizer qualification remain outside this bounded milestone.
Full compiler Go and frozen Zig suites were skipped under the current bounded
qualification policy; compiler and legacy Zig sources are unchanged. This is
library/application qualification, not a new whole-compiler qualification claim.

## Reproduction and integration

Use Concept PR #193 (or a main revision containing it) before building this Oct
branch. The source-root dependency remains `../Concept/libraries/Vulkan`; temporary
feature-branch checkout rules are not added to CI.

```powershell
go run ./tools/prometheus_concept -build-package
cmd /d /c internal\prometheus\native\build_windows_launcher.cmd
$env:PROMETHEUS_VK_VALIDATION = '1'
$env:VK_LAYER_VALIDATE_SYNC = '1'
out/prometheus/native/marionette_tests.exe PrometheusConceptMainTransformerRunsIndependentJointWitnessesAndChains
```

The M11 branches start from current merged mains. Prior migration work is retained
through normal ancestry; review/merge the library dependency before the Oct PR.
