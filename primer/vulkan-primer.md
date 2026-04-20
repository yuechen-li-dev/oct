document:
  name: vulkan-primer
  format: toon
  version: 1

intent:
  summary:
    Define how Vulkan code is allowed to exist in this repository.
    Prevent raw API sprawl, synchronization fanfiction, and lifetime chaos by enforcing a narrow, repeatable backend architecture.
  audience:
    primary:
      LLM authors
    secondary:
      human contributors
  non_goals:
    - This is not a general Vulkan tutorial.
    - This is not a catalog of every Vulkan object, flag, or extension.
    - This is not permission to spread raw Vulkan handles across the codebase.
    - This is not permission to improvise synchronization, lifetime, or destruction policy ad hoc.
    - This is not a license to build a giant abstraction tower that hides Vulkan reality.

goal:
  headline:
    Use boring, centralized Vulkan architecture that keeps ownership, synchronization, and lifetime rules explicit and prevents raw API sprawl across the repository.
  success_criteria:
    - Vulkan stays behind a narrow backend boundary.
    - Ownership and destruction rules are obvious.
    - Frame resources follow a stable repeated pattern.
    - Synchronization policy is centralized and consistent.
    - Resource creation, upload, and destruction follow a small number of established recipes.
    - Validation remains useful because the architecture is explicit instead of magical.
    - Authors do not invent new Vulkan patterns casually.

core_principles:
  - Prefer the dull solution.
  - Prefer explicit lifetime and ownership over implicit convenience.
  - Prefer a small number of repeated patterns over many clever local optimizations.
  - Prefer centralized policy over subsystem improvisation.
  - Prefer validation-friendly code over abstraction theater.
  - Prefer narrow backend boundaries over engine-wide Vulkan leakage.
  - Prefer wrappers that encode ownership and repeated policy, not wrappers that pretend Vulkan does not exist.
  - Do not use a Vulkan feature merely because it exists.
  - Do not make the renderer more "low-level" at the cost of making it less trustworthy.

must_do:
  - Keep Vulkan behind a narrow backend boundary.
  - Keep raw Vulkan handle ownership explicit.
  - Centralize resource creation and destruction policy.
  - Centralize synchronization and resource transition policy.
  - Use stable repeated frame-in-flight patterns.
  - Keep command recording structure predictable and reviewable.
  - Follow existing repository-local Vulkan patterns before introducing a new one.
  - Keep validation enabled during development paths unless there is a documented reason not to.
  - Prefer obvious correctness over clever wrapper design.
  - Make it easy for a reviewer to tell who creates, owns, records, submits, transitions, and destroys each class of Vulkan object.

recommended:
  - Prefer a thin renderer or backend interface above Vulkan.
  - Prefer a small number of backend-owned managers with clear responsibilities.
  - Prefer explicit frame context objects for per-frame resources.
  - Prefer standard upload or staging paths reused consistently.
  - Prefer deferred destruction queues or frame-based retirement patterns when needed.
  - Prefer explicit wrapper types for repeated ownership cases.
  - Prefer one descriptor management pattern per repository area instead of several competing ones.
  - Prefer simple command recording phases with obvious boundaries.
  - Prefer explicit debug naming and validation-friendly diagnostics.
  - Prefer repository-established helper layers such as allocator or loader integrations when already adopted.
  - Prefer explicit timestamp and query usage patterns when GPU timing is needed.
  - Prefer explicit host-side SPIR-V loading patterns that respect byte alignment.
  - Prefer explicit push constant layout discipline shared by host and shader code.

not_recommended:
  - Raw VkDevice, VkCommandBuffer, or other core handles flowing through unrelated high-level systems.
  - Multiple ownership styles for the same class of Vulkan resource.
  - Ad hoc barriers and layout transitions written differently in every subsystem.
  - Temporary bootstrap code quietly becoming permanent architecture.
  - Wrappers that hide cost, ownership, synchronization, or state transitions.
  - Giant god-objects that own the entire renderer without meaningful boundaries.
  - Re-encoding the entire Vulkan object model in custom wrappers for prestige.
  - Per-feature mini-frameworks with their own descriptor, upload, or synchronization rules.
  - Copy-pasted tutorial code without repository-specific ownership and lifetime adaptation.
  - "Just make it work" synchronization patches with no clear policy.
  - Extension and feature usage without a repository-level reason.

restricted:
  - Raw Vulkan handles outside approved backend layers are restricted.
  - Ad hoc synchronization logic outside centralized policy points is restricted.
  - Ad hoc resource layout transition logic is restricted.
  - New wrapper layers are restricted unless they encode repeated ownership or policy clearly.
  - Direct subsystem access to VkDevice, queues, allocators, or command pools is restricted.
  - Per-call-site descriptor pool or command pool invention is restricted.
  - Feature and extension use beyond repository-established needs is restricted.
  - Multi-threaded command recording patterns are restricted unless the repository already standardizes them.
  - Async upload and lifetime tricks are restricted unless the architecture already provides them.
  - Unsafe interop edges and platform glue are restricted to narrow documented boundaries.
  - Ad hoc timestamp query placement and readback policy are restricted.
  - Unaligned SPIR-V typed reads from arbitrary byte buffers are restricted.
  - Push constant layout assumptions without explicit host and shader agreement are restricted.

default_patterns:
  backend_boundary:
    headline:
      Vulkan is a backend, not an ambient fact of the whole repository.
    rules:
      - Keep Vulkan-specific types and logic inside defined backend layers.
      - High-level rendering code should depend on renderer concepts, not raw Vulkan handles.
      - Do not let unrelated systems reach into Vulkan backend internals for convenience.
      - If a type exists only to support Vulkan, keep it in Vulkan-facing layers.

  ownership_and_lifetime:
    headline:
      Every Vulkan object must have a clear owner and a clear destruction rule.
    rules:
      - Make ownership explicit in types and module boundaries.
      - Standardize destruction timing for each resource class.
      - Do not let destruction policy vary casually by subsystem.
      - Prefer backend-owned resource wrappers when they encode real lifetime rules.
      - If an object's lifetime is unclear, simplify the design before adding helpers.

  device_and_resource_creation:
    headline:
      Resource creation must follow repository recipes, not local improvisation.
    rules:
      - Centralize instance, device, allocator, and major resource creation policy.
      - Reuse standardized creation helpers for common buffers, images, samplers, and views.
      - Do not duplicate setup logic with small variations across the codebase.
      - Keep feature and extension enabling decisions centralized.

  frame_resources:
    headline:
      Per-frame resources should follow one stable pattern.
    rules:
      - Use explicit frame context or frame resource objects.
      - Keep fences, semaphores, transient allocators, and temporary buffers organized per established frame structure.
      - Do not mix permanent resources and frame-local scratch state carelessly.
      - Make frame ownership and reset points obvious.

  command_recording:
    headline:
      Command recording should be predictable and phase-oriented.
    rules:
      - Use a stable recording flow for frame begin, pass recording, submission, and frame end.
      - Keep command buffer ownership and reuse policy explicit.
      - Do not let arbitrary systems record commands whenever they feel like it.
      - Keep render pass, pipeline, descriptor, and resource binding order consistent where practical.

  synchronization:
    headline:
      Synchronization policy must be centralized and boring.
    rules:
      - Use a small number of repository-approved synchronization patterns.
      - Do not invent barriers or stage masks ad hoc at random call sites.
      - Keep resource transition and visibility rules in known locations.
      - Prefer explicit documented state-transition helpers when the repository uses them.
      - If synchronization is unclear, simplify the pass or resource model before adding more barriers.
    guidance:
       - Plausible-looking synchronization is not good enough.
       - The repository should have a synchronization doctrine, not a pile of individually reasonable guesses.

  timestamps_and_queries:
    headline:
      GPU timing and query usage must follow explicit recipes.
    rules:
      - Use a small number of repository-approved timestamp and query patterns.
      - Record timestamps at stages that actually bracket the GPU work being measured.
      - Reset queries according to a consistent backend policy before reuse.
      - Do not read query results until the backend has established that the work is complete.
      - Keep host-side timing semantics distinct from GPU-side timing semantics.
    guidance:
      - Plausible timestamps can still measure the wrong thing.
      - If a timing path is unclear, simplify the measurement scope before adding more query logic.

  shader_module_loading:
    headline:
      Shader loading must respect SPIR-V representation and alignment.
    rules:
      - Do not reinterpret arbitrary byte buffers as uint32_t words without proving alignment.
      - Use a repository-approved SPIR-V loading path that produces correctly aligned code data.
      - Keep shader-module creation policy centralized.
      - Validate shader byte length and representation before module creation.
    guidance:
      - Shader loading is low-level code and low-level code must still be boring.
      - A plausible cast is not a safe loading strategy.

  push_constants:
    headline:
      Push constant layout must be explicit and shared between host and shader expectations.
    rules:
      - Keep push constant structs simple, explicit, and size-aware.
      - Do not assume host struct layout is correct without matching shader-side layout rules.
      - Centralize push constant range definitions where practical.
      - Prefer one stable push constant recipe per pipeline family instead of many local variations.
    guidance:
      - Push constants are easy to make silently wrong.
      - Layout discipline matters more than convenience here.

  uploads_and_transfers:
    headline:
      Upload and transfer paths should be standardized.
    rules:
      - Use an established staging or transfer pattern for resource uploads.
      - Keep ownership and retirement of staging resources explicit.
      - Do not create one-off upload flows for each feature.
      - Make transfer completion and resource readiness rules obvious.

  descriptors_and_pipelines:
    headline:
      Descriptor and pipeline management should be consistent, not improvised.
    rules:
      - Reuse repository-standard descriptor allocation and update patterns.
      - Do not invent new descriptor lifetime models casually.
      - Keep pipeline creation and caching policy centralized where practical.
      - Avoid feature-local descriptor management systems that duplicate backend infrastructure.

  wrappers:
    headline:
      Wrap policy and ownership, not reality itself.
    rules:
      - Use wrappers when they encode repeated lifetime, destruction, or setup rules.
      - Do not wrap every Vulkan object just to avoid seeing Vulkan names.
      - Do not hide synchronization, allocation, or submission cost behind pleasant method names.
      - Prefer thin explicit wrappers over fake-engine object mythology.

  validation_and_debugging:
    headline:
      Validation is part of development, not an optional accessory.
    rules:
      - Keep validation layers enabled in development-oriented configurations when practical.
      - Surface validation failures clearly.
      - Use debug names and labels where the repository already supports them.
      - Do not suppress warnings by making the architecture less explicit.

  portability_and_features:
    headline:
      Features and extensions should be deliberate and centralized.
    rules:
      - Enable only features and extensions the repository actually needs.
      - Keep capability checks centralized.
      - Do not let individual subsystems negotiate backend capability independently.
      - Isolate platform-specific surface or windowing glue behind narrow boundaries.

author_operating_rules:
  - First inspect nearby backend code and mirror established Vulkan patterns.
  - Do not introduce a second pattern for a problem the repository already solved.
  - Do not widen the Vulkan surface area of the repository during implementation.
  - Do not infer that direct raw Vulkan access is preferred just because it is possible.
  - When multiple solutions are valid, choose the one with the clearest ownership.
  - When multiple solutions are valid, choose the one with the most centralized synchronization policy.
  - When multiple solutions are valid, choose the one with the fewest distinct lifetime rules.
  - If a wrapper makes review harder or hides cost, synchronization, or destruction, avoid it.
  - If unsure, choose the simpler and more explicit backend shape.
  - Do not paste tutorial architecture into the repository without adapting it to repository ownership and boundary rules.
  - Do not solve a local Vulkan problem by leaking Vulkan deeper into the engine.
  - Do not mistake validation silence for architectural quality.

decision_rules:
  - Prefer code that is obviously correct over code that is merely plausible-looking Vulkan.
  - Prefer one boring repeated backend pattern over several elegant competing ones.
  - Prefer local explicit ownership over shared backend mystery.
  - Prefer centralized synchronization policy over distributed barrier folklore.
  - Prefer explicit resource lifecycle rules over convenience helpers with hidden consequences.
  - Prefer wrappers that help the reviewer reason better, not wrappers that erase important facts.
  - If the clever solution and the boring solution are both valid, choose the boring solution.

examples:
  backend_boundary_examples:
    headline:
      Show Vulkan quarantine contrasted with raw-handle leakage into higher-level systems.

  ownership_examples:
    headline:
      Show clear backend-owned resource lifetimes contrasted with scattered destruction and unclear owners.

  frame_resource_examples:
    headline:
      Show stable per-frame context patterns contrasted with ad hoc frame-local state.

  synchronization_examples:
    headline:
      Show centralized transition and barrier policy contrasted with call-site improvisation.

  upload_examples:
    headline:
      Show standardized staging or transfer flows contrasted with one-off resource upload logic.

  wrapper_examples:
    headline:
      Show thin ownership-and-policy wrappers contrasted with reality-hiding abstraction towers.

  validation_examples:
    headline:
      Show validation-friendly development setup contrasted with warning suppression and opaque failure handling.

review_checklist:
  ask_before_merging:
    - Does this keep Vulkan behind the backend boundary?
    - Is ownership of every new Vulkan object clear?
    - Is destruction timing clear and consistent with repository policy?
    - Does this reuse existing synchronization and transition patterns?
    - Did this change invent a new wrapper, upload path, descriptor policy, or frame pattern without strong reason?
    - Would a reviewer understand who records, submits, and retires the relevant commands and resources?
    - Does this make validation easier to trust or harder to trust?
    - Would the simplest valid backend version of this change look substantially different?
  reject_if:
    - The change leaks raw Vulkan concepts deeper into unrelated systems.
    - The change introduces ad hoc synchronization or layout transitions.
    - The change introduces unclear ownership or destruction timing.
    - The change invents unnecessary wrapper layers.
    - The change duplicates backend policy in feature-local code.
    - The change makes review, debugging, validation, or future refactoring meaningfully harder.

closing:
  message:
    This repository uses a narrow Vulkan architecture on purpose.
    The goal is not to show how much Vulkan you know.
    The goal is to produce explicit, centralized, validation-friendly backend code that is easy to review and hard to accidentally turn into radioactive spaghetti.
