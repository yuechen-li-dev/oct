# Prometheus R2a — real batch contract

Status: design/report only. No runtime source, routing, ABI, tests, or public behavior is changed.

## Executive summary

R2 preserves P11's durable batch semantics, not P11's implementation. M31 is the production-authoritative engine: a deterministic one-compute-queue refill loop on the M29 persistent physical ring, with M30 task ownership and M30a quarantine/reap. Current dispatch confirms that zero flags (and only the encoded fail-entry seam) call prom_sgemm_batch_refill_ring. Ordinary nonzero P11 controls call the retained executor, which may submit empty command buffers and obtains results through batch_reference_sgemm. That is not production SGEMM authority.

The target contract is immutable per-entry planning, stable entry identity, entry-order admission, bounded ring depth, deterministic first failure, stop admission, safe drain/reap, staged output, atomic commit, and entry-order commit. Logical lane metadata remains useful for reproducibility but is neither a CPU thread, Vulkan queue, nor physical slot.

## Evidence and present call path

This audit read R0/R1; P11 batch reports M6–M8, M10–M11, M13–M14, M16–M17, M19–M20; M29/M30/M30a/M31; current P11/M31 Marionette tests; public declarations; current implementation; and both R0 JSON maps. P11 M3 typed-arena tests are not batch-executor contracts and remain separately owned by allocation work.

| Route | Result authority | Physical execution | Diagnostics |
|---|---|---|---|
| zero flags / fail-entry mask | M31 real Vulkan SGEMM | M29 shared ring, M30 records, M30a reaping | M31 fields are real evidence |
| other P11 flags | CPU reference oracle | optional P11 empty submits | P11 simulator fields |

Relevant implementation symbols are prom_reactor_runtime_sgemm_batch_impl, prom_sgemm_batch_refill_ring, prom_batch_plan, batch_worker_partition, batch_reference_sgemm, P11 batch_worker helpers and prom_batch_worker_resources, M29 ring helpers, M30 record/poll helpers, and M30a reaping.

## P11 inventory and classification

Classification is exclusive: **U** preserve unchanged, **R** preserve with redefined semantics, **T** testing-only, **M** replace with M31-native behavior, **X** reject in production, **D** delete with P11, **?** user decision.

| P11 behavior | Current meaning / symbols | Tests / diagnostics | Production value and M31 equivalent | Class | Future meaning and proof |
|---|---|---|---|---|---|
| validation | P11 preflight / prom_batch_plan | M6 atomic, M7 single-entry; failure fields | semantic; M31 validates before admission | U | validate every entry and staging allocation before Vulkan admission; invalid-index/no-submit proof |
| stable entry identity | entry_id, plan generation | M6–M11 failure/arrays | semantic; M31 index identity | U | caller-order index immutable for batch lifetime |
| deterministic planning | P11 selector facts copied into plans | M6/M7 planning diagnostics | semantic; selector payload is machinery | R | R2b immutable plan with request facts/policy snapshot; no replan proof |
| round-robin partition | batch_worker_partition, bits 0–7 | M6/M8 | useful metadata; M31 retains metadata | R | logical lane equals entry ID modulo planned width; map test only |
| contiguous partition | bit 8 / partition helper | M7/M8 | useful planning policy | R | supported planning hint, never physical affinity |
| worker count/caps | low byte, hardware/memory caps | M7/M8/M11 | P11 caps fake workers; M31 caps diagnostic width | R | requested logical width to planned logical width; no queue/thread claim |
| lane rounds | batch worker executor | M8 | simulator scheduler | D | entry-order central refill replaces it |
| CPU threads/bridge | P11 thread bridge | M10/M11/M20 | no M31 correctness value | X | no production CPU-thread-count promise |
| event rings/overflow | worker event structs/drain | M6/M8/M11 | attribution is semantic; rings/overflow are not | R | bounded trace from real facts; trace exhaustion must not fail valid work |
| first failure | shared P11 failure state | M6–M11/M14/M17/M20 | semantic; M31 needs explicit tie rule | R | minimum (entry ID, phase rank, observation sequence) among terminal failures |
| stop admission | failure gates | M7/M11/M17 | semantic; M31 already stops refill | U | no admission after selected failure |
| drain/destruction | joins, waits, queue drain | M11/M14/M17/M20 | semantic; M30a provides it | R | resolve submitted ownership; quarantine uncertainty; reusable/fatal proof |
| staged atomic commit | P11 staging/copy | M6/M7/M11/M17/M19/M20 | semantic; M31 already stages | U | caller C unchanged on failure |
| ordered commit | entry-ID copy loop | M7/M11/M19 | semantic; M31 already does it | U | ascending entry-ID commit despite completion order |
| skip/cancellation | non-started plans, drain events | M7/M11 | useful admission state; no API | R | skipped-not-admitted versus submitted-and-drained; no new cancel API |
| worker-local pools/fences/slots | P11 resource structs | M13/M14/M19 | duplicate physical ownership | D | M29 slot owns command/fence/descriptor/query; no lane affinity |
| empty Vulkan submit | P11 physical path | M14/M16/M17/M20 | not SGEMM authority | X | reject as production evidence; delete |
| queue topology/mapping | P11 topology fields | M13/M16/M17/M20 | one production compute queue only | X | unsupported multi-compute requests error; never emulate |
| transfer not compute lane | topology gate | M16 | correct distinction; M31 facts exist | U | retain as physical diagnostic distinction |
| two slots per worker | P11 slot runtime | M19/M20 | duplicate lifecycle | D | global ring depth, no lane-owned slot |
| invalidation/masks | P11 slot masks | M19/M20 | ownership safety valuable, masks not | M | M29 state/generation and M30a quarantine |
| test injections | flags/test flags | M14/M17/M20 | deterministic proof only | T | testing API/config; never ordinary production flags |
| CPU reference result | batch_reference_sgemm | M20 oracle | valuable only as oracle | T | retain test oracle, never native result authority |
| diagnostics truth | P11 counters/modes/topology | M7/M11/M20 | principle semantic, many fields not | R | source-label plan/ring/trace facts; v2 later |
| repeated reuse | per-batch reset | M11/M17/M20 | semantic reusable runtime | U | nonfatal batch releases/reaps ownership before next submit |
| feedback | P11 deferred drain | M7/M20 | M30 feedback is real | M | valid completion only; ordered skipped feedback where M30a requires |

### Semantics versus machinery

Useful semantics are deterministic planning, stable identity, bounded admission, first-failure stability, stop admission, safe drain, no partial commit, ordered commit, event attribution, and reproducible scheduling.

Obsolete machinery is worker-local command pools/fences/physical slots, empty command buffers, CPU result authority, duplicate thread bridge, fake multi-queue execution, queue-count-as-worker-count, and duplicate physical lifecycle state. A test reference does not make machinery a production contract.

## Worker and topology semantics

| Term / v1 field | P11 meaning | M31 now | Future vocabulary / compatibility |
|---|---|---|---|
| requested workers, bits 0–7 | intended workers/lanes | metadata | **requested logical scheduling width**; retain field but never threads/queues |
| effective workers | fake cap result | diagnostic cap | **planned logical width**, deterministically defined |
| worker/lane ID | partition owner | worker-array metadata | **logical partition/lane identity**, never owner |
| queue count/topology/mapping | fake or empty-submit lanes | one compute queue | **physical compute queue count**, observed fact only |
| queue family | classifier | compute/transfer discovery | physical family fact; transfer is not compute width |
| physical slot | two per P11 worker | M29 slot | **ring depth** / physical ring slot ID |
| logical slot | local lifecycle | none | reject as production batch concept |
| partition count | implied workers | metadata | planned logical width |
| scheduling width | conflated concepts | absent | requested width, planned width, ring depth, max in-flight distinct |
| observed in-flight | fake worker slots | M29 outstanding/max | **observed maximum in-flight**, physical evidence |

Invariant: logical planning metadata != CPU worker thread != Vulkan queue != physical ring slot. Do not silently redefine a v1 field. Worker count remains a compatibility planning hint; topology/resource fields become deprecated compatibility diagnostics and must not claim multiplicity.

## Public flag audit

Every nonzero P11 batch control is listed below. Ordinary nonzero controls currently divert to P11 except the encoded fail-entry mask, specially allowed through M31.

| Flag | Numeric value | Current behavior/tests | M31 mapping | Target |
|---|---:|---|---|---|
| requested workers | 0x000000ff | P11 workers/lanes, M6–M20 | logical planning width | supported planning hint; approval required |
| PARTITION_CONTIGUOUS | 0x00000100 | M7/M8 | deterministic lane metadata | supported planning hint |
| FAIL_AFTER_FIRST_SUBMIT | 0x00000200 | M31/P11 injection | post-submit seam | testing-only; move off ordinary ABI |
| test HW cap | 0x00003c00 | M7/M16 fake cap | none | testing-only; production reject |
| test arena scale | 0x0000c000 | P11 memory cap | none | testing-only; production reject |
| test event capacity | 0x003f0000 | M6/M11 overflow | none | testing-only; production reject |
| dual failure | 0x00400000 | M11 race | reducer injection | testing-only |
| delay entry 0 | 0x00800000 | M11 ordering | completion seam | testing-only |
| fail entry | 0xff000000 | P11/narrow M31 injection | selected failure seam | testing-only now; later testing API |

No ordinary production flag may select CPU result production. R2d supports only approved width/partition hints on M31; legacy combinations become explicit unsupported errors, never P11 fallback.

### Diagnostics field mapping

The current PrometheusSgemmBatchDiagnostics ABI mixes two incompatible schemas. R2 must keep v1 layout stable but only populate an observed or explicitly compatibility-labelled fact.

| Field family | Source today | R2 classification |
|---|---|---|
| last_batch_entry_count, requested_workers, effective_workers, partition_policy, plan_generation | P11 plan / M31 local arrays | preserve, redefine workers as logical width, label plan-derived |
| failed_entry_id, failed_worker_id, failure_stage/detail/count, first_failure_stable, batch_state, output_committed | P11 shared state / M31 result | preserve; worker ID becomes logical lane; source is failure reducer/commit result |
| worker assigned/completed/event/submit counters and active mask | P11 worker states; M31 lane arrays | preserve only as logical-lane aggregates where meaningful; no physical worker interpretation |
| execution_mode, real_worker_thread_count, serialized bridge counters | P11 bridge | deprecated compatibility values until diagnostics v2; do not use as production execution claims |
| worker command pool/buffer/fence/queue/resource fields | P11 local resources | no M31 equivalent; freeze as legacy-only/zero or explicit unavailable in v1, remove in v2 |
| topology/mapping, compute queue counts, fallback reasons | P11 topology simulation | physical queue facts only; unsupported topology is an error, not a fallback mode |
| P11 slot arrays, masks, slot cap/refill/poll counters | P11 worker slots | legacy-only; M29 ring diagnostics replace them |
| physical_ring_depth, current/max in-flight, submit/poll/wait/ring-full/refill/query counters | M29 shared ring | preserve as observed physical facts |
| quarantine/reap and feedback committed/skipped | M30a/M30 | preserve as observed lifecycle and ordered-feedback facts |
| M31 submission sequence, physical slot ID, completion status/duration/order, commit order | M31 | preserve as real per-entry evidence, bounded by existing ABI arrays |

## M31 capability map

M31 already guarantees real Vulkan SGEMM, M29 shared physical ring, bounded in-flight depth, refill, stable entry ownership, stop admission, M30a drain/reap, staged outputs, atomic and entry-ordered commit, submission/slot/completion evidence, and M30 P14/P15 ordered feedback.

| P11 semantic | M31 status | R2 action |
|---|---|---|
| validation, identity, admission, staging, atomic ordered commit | present | name and lock down with plan tests |
| logical partition metadata | lightweight worker array | formalize; remove resource implication |
| bounded depth/refill/evidence | stronger than P11 | retain and test |
| first failure | present but implicit order | explicit reducer + out-of-order test |
| feedback | present | prove completed-only / skipped-on-failure |
| threads, multi-queue, worker resources | absent by design | do not port |
| public trace, plan diagnostics, diagnostics v2 | absent | bounded R2c proposal; ABI decision later |
| cancellation | absent | do not invent |

## Target real batch contract

1. A valid request has a live runtime, entry array/count, non-null A/B/C, positive dimensions, and checked counts. Validate and allocate caller-output staging before first admission.
2. Each entry receives immutable caller-order entry ID and plan: dimensions, identities, output size, policy snapshot, logical lane, plan generation. Never reassign/replan.
3. Admit ascending entry ID only while an empty M29 slot and M30 task record exist. Ring depth, not lane count, bounds physical work.
4. Each submission owns one ring-slot generation, task buffers, and sequence. No slot is lane-owned.
5. Completion is authoritative only after physical fence/query lifecycle. Copy to staging, not caller C.
6. A terminal failure is reduced once by the deterministic rule; stop admission; drain/reap or quarantine every admitted item.
7. Commit only when every entry has valid staged output, ascending entry ID. Failure changes no caller output.
8. Diagnostics separate planned logical facts, observed ring facts, and completed evidence. Unsupported topology errors explicitly.

## Event model

No event framework is added in R2a.

| Event | Required identity / source | Kind | Sequence? | Visibility |
|---|---|---|---|---|
| planned | batch, entry, plan generation, logical lane / immutable plan | logical | no | internal; aggregate public |
| admitted | entry, ring slot generation / allocation | bridge | no | internal |
| submitted | entry, slot/generation / successful Vulkan submit | physical | yes | public evidence |
| completed | entry, slot/generation / harvested fence/query/result | physical | yes | public evidence |
| failed | entry, stage/detail/reducer rank | bridge | if submitted | public summary, internal trace |
| skipped | never-admitted entry after selected failure | logical | no | internal/aggregate |
| drained | submitted entry / safe retire or quarantine fact | physical | yes | internal/counters |
| committed | entry copied to caller output | logical result | no | public ordered evidence |

## Failure contract

First failure is the minimum (entry ID, phase rank, observation sequence) among terminal failures known by finalization. Selection freezes identity/stage/detail and stops admission. Every admitted item is drained, reaped, or M30a-quarantined; no physical ownership is recycled while uncertain.

There is no partial user-visible output commit. Original caller outputs stay intact on failure. Nonfatal allocation, recording, submit, and observation failures leave the runtime reusable once ownership is safe. Device loss/global unsafe marks the runtime unsafe and later submit fails explicitly. P14/P15 feedback commits only valid completed work; M30a advances one terminal skipped feedback where required. Injection is testing-only.

P11 proves desired atomicity and drain through simulator workers. M31 proves real output authority, ring ownership, quarantine, and ordered feedback. R2c must prove their combined real-path contract.

## P11 test migration map

This covers every batch FACT in reactor_p11_m6_batch_tests.cpp. Rewrite means real-M31 authority test; test-only means temporary P11 simulator coverage; delete means no durable contract.

| Test | Intended proof | Disposition |
|---|---|---|
| M6 BatchRoundRobinAndDiagnostics | round-robin diagnostics | rewrite immutable M31 lane map |
| M6 BatchFailureIsAtomicAndUncommitted | atomicity | rewrite real post-submit failure + sentinels |
| M6 BatchCriticalEventOverflowFailsExplicitly | event ring | delete; optional trace-truncation truth |
| M7 BatchSingleEntryMatchesSingleSgemmPath | correctness | rewrite M31 Vulkan vs CPU oracle |
| M7 ContiguousPartitionAssignmentDeterministicAndOrderedCommit | map/commit | rewrite M31 map + order |
| M7 WorkerCapsExposeHardwareAndMemoryReasons | fake caps | delete; width validation |
| M7 ZeroWorkerMemoryFailureAndSingleQueueConservativeReason | fake caps/topology | delete |
| M7 LateFailureAndDominatusGapRemainExplicit | failure/feedback | rewrite real drain + skipped feedback |
| M7 DiagnosticsTruthfulnessForSuccessfulBatch | truth | rewrite plan/ring/commit sources |
| M8 MultiWorkerAssignmentAndLaneExecutionDiagnostics | simulator lanes | rewrite map only; delete mode claim |
| M8 ContiguousAssignmentAndFailureDrainAcrossWorkers | map/drain | rewrite map + M31 drain |
| M10 RealThreadsSerializedVulkanModeAndDiagnostics | thread bridge | delete |
| M10 FirstFailureWinsAndLaneFallbackStillAvailable | first failure/fallback | rewrite reducer; delete fallback |
| M10 RealThreadEnablementGateIsExplicitAndForceLaneWins | injection gate | test-only then delete |
| M11 ConcurrentFailuresPreserveDeterministicFirstFailure | race ordering | rewrite real ordered injection |
| M11 RealThreadCriticalOverflowFailsWithoutCommit | event ring | delete |
| M11 FailureWhileOtherWorkerEmitsDrainsSafely | concurrency drain | rewrite multiple outstanding ring slots |
| M11 OutOfOrderCompletionStillCommitsInEntryOrder | order | rewrite real completion/commit evidence |
| M11 RepeatedBatchesResetPerBatchState | reuse | rewrite M31 reusable lifecycle |
| M11 RuntimeDestroyAfterFailedRealThreadBatchIsSafe | destruction | rewrite M31/M30a destruction |
| M11 WorkerRestrictionsRemainEnforcedAndDiagnosticsTruthful | worker limits | delete limits; rewrite truth |
| M13 PerWorkerCommandResourceIdentityAndQueueMappingDiagnostics | resources | delete |
| M13 WrongOwnerCommandResourceUseRejected | ownership | rewrite M29 slot generation mismatch |
| M14 PhysicalWorkerCommandResourcesModeAndSerializedSubmitPreserved | empty submits | delete |
| M14 FailureHooksPreserveFirstFailureAndCleanup | injection cleanup | rewrite M31 test seams |
| M16 SingleQueueFallsBackToSerializedBridge | bridge | delete |
| M16 IndependentTwoQueueSelectsTrueMultiQueueWithHook | fake multi-queue | delete; future new suite |
| M16 MemoryCapBlocksTrueMultiQueue | fake cap | delete |
| M16 TransferQueueNotCountedAsComputeLane | topology distinction | rewrite device diagnostic test |
| M17 FailureAfterSubmitDrainsAndStaysUncommitted | drain/atomicity | rewrite real M31 |
| M17 WorkerFenceResetAndWaitFailuresRemainWorkerScoped | worker fences | rewrite slot failure classes, no worker scope |
| M17 DrainTimeoutMarksUnsafeToReuse | unsafe runtime | rewrite M30a class contract |
| M17 DeviceLostDominatesAndMarksUnsafe | device loss | rewrite when seam exists; otherwise honest gap |
| M17 RepeatedTrueMultiBatchesAndMappingStayStable | multi-queue map | delete; rewrite reuse only |
| M17 FallbackReasonsRemainTruthfulAcrossGateTransitions | simulator reason | delete; unsupported-topology test |
| M19 WorkerLocalTwoSlotOwnershipPublished | worker slots | delete |
| M19 SlotBudgetCapReported | worker budget | delete; ring-depth evidence |
| M19 BothSlotsUsedBySingleWorker | local slots | delete |
| M19 PreSubmitFailureUpdatesFailedAndAttentionMasks | failure state | rewrite M31 pre-submit cleanup |
| M19 InvalidatedReadySlotIsRejected | lifecycle safety | rewrite M29 generation/quarantine |
| M19 OrderedOutputCommitAcrossSlots | ordered commit | rewrite M31 evidence |
| M19 S2CompatibilityAcrossExecutionModes | P11 modes | delete |
| M20 ModeMatrix / WorkloadChurnMatrix | modes/churn | delete modes; rewrite bounded M31 stress |
| M20 FailureMatrix / FailureMatrix_DrainTimeoutSlowCase | failure classes | rewrite M31/M30a matrix |
| M20 RepeatedLifecycle / DominatusTruthfulness / OutputOracle | lifecycle/feedback/correctness | rewrite M31 tests |
| M20 BatchThreadStartFailureJoinsOnlyStartedWorkers | thread cleanup | test-only then delete |
| M20 StagedOutputCleanupOnPlanFailure | preflight staging | rewrite M31 preflight/sentinel |
| M20 LaneSlotLifecycleAdvancesToInFlightOrComplete | simulator slots | delete |

PrometheusSgemmBatchRefillRing is the production-suite seed. Empty-submit and CPU-fallback tests must never be relabeled as authority tests.

## R2b–R2e sequence

| Slice | Prerequisites / behavior | Tests | Rollback boundary / deletion gate |
|---|---|---|---|
| R2b | immutable SGEMM plan beside unchanged M31; no routing change | deterministic plan map + current M31 hardware lane | revert plan integration; no deletion until no-replan parity |
| R2c | port planning/refill/event/failure semantics into M31 | real correctness, depth, late failure, order, atomic output, reuse/quarantine | retain P11 route; gate on authority coverage |
| R2d | route approved flags to M31; testing API or explicit rejection for legacy controls | flag matrix, unsupported errors, ABI diagnostics, M31 lane | routing-only rollback; P11 test-only until compatibility decision |
| R2e | delete P11 diversion, empty submits, native CPU authority, worker-local Vulkan resources; retain CPU oracle | no P11 public route; M31/M29/M30a lifecycle/failure and source-search checks | only after R2d evidence and approval |

## Decisions requiring approval

| Decision | Recommended answer |
|---|---|
| worker count | requested logical scheduling width; never CPU threads/queues |
| public logical partitioning | retain round-robin/contiguous hints for one v1 window; typed option in v2 |
| separate-compute-family enum | preserve numeric tombstone, deprecate, reject as batch request; no implementation reference |
| multi-queue | separate future real-authority roadmap; no P11 implication |
| nonzero flag window | R2d supports only approved width/partition; rejects other legacy/test values |
| temporary P11 simulator | test-only through R2d, delete R2e after migrated suite |
| diagnostics v2 | design R2c; ship before/with R2e if external consumers require ring facts |

## Recommendation

Approve R2b with this contract and vocabulary. Preserve behavioral invariants, prove M31 as authority, treat P11 thread/topology/resource controls as legacy test machinery, and require explicit approval before changing v1 worker/partition compatibility. Consolidate authority first, delete the simulator second, and extract modules only after ownership is singular.
