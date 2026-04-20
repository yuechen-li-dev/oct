document:
  name: cpp-primer
  format: toon
  version: 1

intent:
  summary:
    Define the narrow C++ subset allowed in this repository.
    Reduce ambiguity, avoid foot-cannons, and keep code portable, explicit, and locally testable.
  audience:
    primary:
      LLM authors
    secondary:
      human contributors
  non_goals:
    - This is not a general C++ tutorial.
    - This is not a showcase of every valid C++23 feature.
    - This is not a license to invent a new house dialect.
    - This is not an excuse to import clever patterns from random C++ codebases.

goal:
  headline:
    Use boring, explicit, portable C++23 that keeps ownership, control flow, and runtime cost obvious.
  success_criteria:
    - Code is easy to review locally.
    - Refactors stay localized.
    - Unit tests are meaningful.
    - The same code compiles cleanly on MSVC, Clang, and GCC.
    - Authors do not introduce cleverness unless clearly required.

core_principles:
  - Prefer the dull solution.
  - Prefer explicit behavior over hidden behavior.
  - Prefer local reasoning over distributed magic.
  - Prefer portability over vendor tricks.
  - Prefer standard library tools over custom reinvention.
  - Optimize late and locally, not preemptively and globally.
  - Do not use a C++ feature merely because it exists.
  - Do not make the codebase more "advanced" at the cost of making it less trustworthy.

must_do:
  - Target C++23.
  - Keep code portable across MSVC, Clang, and GCC.
  - Use RAII for resource management.
  - Make ownership explicit at the point of use.
  - Write code whose failure modes are visible in tests.
  - Keep functions and types small, narrow, and unsurprising.
  - Follow existing repository-local patterns before introducing a new one.
  - Keep performance-sensitive work localized so it can be optimized without infecting unrelated code.
  - Prefer obvious correctness over idiomatic cleverness.
  - Keep data flow and control flow easy to inspect in one pass.

recommended:
  - Prefer value types by default.
  - Prefer stack allocation unless heap allocation is clearly needed.
  - Use std::unique_ptr<T> for owning heap allocation.
  - Use T* for nullable non-owning access.
  - Use T& for non-null non-owning access.
  - Prefer composition over inheritance.
  - Prefer simple structs and classes with obvious responsibilities.
  - Prefer std::vector, std::string, std::array, std::span, std::optional, and std::variant where they fit naturally.
  - Prefer std::expected<T, E> for fallible operations that need explicit error detail at the call site.
  - Prefer enum class over unscoped enum.
  - Prefer simple free functions when a class adds no real invariant or state value.
  - Prefer straightforward control flow over abstraction-heavy indirection.
  - Prefer const when it clarifies intent and stabilizes interfaces.
  - Prefer narrow interfaces with explicit inputs and outputs.

not_recommended:
  - Deep inheritance hierarchies.
  - Virtual dispatch as a default design habit.
  - Clever template abstractions for small amounts of duplication.
  - Operator overloading outside clear value-semantics cases.
  - Macros used as an abstraction layer.
  - Hidden global state.
  - Static initialization magic.
  - Header-only sprawl without a strong reason.
  - Ad hoc concurrency.
  - Fluent APIs that hide cost, ownership, or control flow.
  - Generic utility layers that the repository does not already need.
  - Abstractions introduced "for flexibility" before a real use case exists.
  - Code that saves a few lines while making behavior harder to inspect.

restricted:
  - Exceptions are banned.
  - Raw owning pointers are banned.
  - Manual new and delete in normal code are banned.
  - std::shared_ptr<T> is restricted to rare, justified shared-lifetime cases.
  - std::weak_ptr<T> is restricted because it usually implies shared ownership complexity.
  - RTTI and dynamic_cast are restricted.
  - Multiple inheritance is restricted.
  - Template metaprogramming, SFINAE acrobatics, and compile-time framework building are restricted.
  - Explicit object parameters ("deducing this") are banned.
  - Deep std::ranges/std::views pipelines are restricted; only short local pipelines with obvious lifetime safety are acceptable.
  - Moving ordinary runtime logic into constexpr/meta machinery without a documented reason is restricted.
  - std::mdspan is restricted to cases that truly require multidimensional non-owning views, with local layout/data-contract documentation.
  - Compiler-specific extensions, intrinsics, and vendor lock-in patterns are restricted.
  - Macros are restricted to unavoidable low-level use such as platform or compiler glue and include control.
  - Hidden singleton patterns are restricted.
  - Custom allocators are restricted unless the repository already has a clearly established low-level allocation strategy.
  - reinterpret_cast is restricted.
  - Unaligned typed reads from byte storage are banned.
  - Implicit narrowing conversions in normal code are restricted.
  - Silent signed and unsigned mixing is restricted.

default_patterns:
  error_handling:
    headline:
      Use explicit return-based error handling.
    rules:
      - Do not throw exceptions.
      - Make failure an explicit part of the function signature when failure is expected.
      - Prefer std::optional<T> when absence is the only failure mode.
      - Prefer a result or status style when callers need failure information.
      - Reserve assertions for violated invariants, not routine runtime conditions.
      - Do not hide failure behind side channels, logging, or surprising global state.
    guidance:
      - Use return values to make success and failure visible at the call site.
      - Favor patterns that unit tests can exercise directly and deterministically.

  ownership:
    headline:
      Ownership must be obvious without guesswork.
    rules:
      - Prefer value semantics first.
      - Use std::unique_ptr<T> for exclusive heap ownership.
      - Treat raw pointers and references as non-owning only.
      - Do not transfer ownership implicitly.
      - Do not hide lifetime behind convenience abstractions.
      - Do not return or store borrowed references unless the lifetime relationship is obvious and safe.
    guidance:
      - A reader should be able to tell who owns an object from local code, not from detective work.
      - If ownership is ambiguous, the design is wrong.

  type_design:
    headline:
      Types should be small, boring, and easy to reason about.
    rules:
      - Prefer plain data plus clear functions over architecture theater.
      - Prefer composition over inheritance.
      - Use inheritance only when runtime polymorphism is genuinely needed.
      - Mark implementation classes final when extension is not intended.
      - Avoid deep or fragile class hierarchies.
      - Do not build class taxonomies for hypothetical future flexibility.
    guidance:
      - A type should have a clear reason to exist.
      - If a struct and a few functions solve the problem cleanly, that is usually the correct answer.

  templates:
    headline:
      Templates may remove repetition, but may not introduce ritual magic.
    rules:
      - Use templates for straightforward generic code only.
      - Do not use templates to impress the compiler.
      - Do not build mini-frameworks out of type traits and indirection.
      - Do not use template tricks where a normal function or class would be clearer.
      - Prefer ordinary overloads or simple specialization only when truly necessary.
    guidance:
      - Templates are a tool for reducing obvious duplication.
      - They are not permission to turn the codebase into compile-time performance art.

  interfaces:
    headline:
      Interfaces should expose intent, not cleverness.
    rules:
      - Keep signatures explicit.
      - Avoid surprise allocation, surprise ownership transfer, and surprise work.
      - Prefer narrow APIs with obvious preconditions and outputs.
      - Use names that describe behavior plainly.
      - Keep side effects visible.
    guidance:
      - Callers should not need to guess about lifetime, cost, or failure behavior.

  integer_safety:
    headline:
      Integer conversions and size arithmetic must be explicit.
    rules:
      - Do not mix signed and unsigned integers casually.
      - Do not rely on implicit narrowing conversions.
      - Use explicit casts only when the range and intent are clear.
      - Be careful with multiplication, byte counts, element counts, and buffer sizes.
      - Prefer checked reasoning over convenient arithmetic when values cross API or allocation boundaries.
    guidance:
      - Size and count bugs often look ordinary in review and still produce wrong behavior.
      - If the integer story is ambiguous, make it explicit.

  low_level_memory_and_casts:
    headline:
      Low-level byte handling must respect alignment and aliasing rules.
    rules:
      - Do not reinterpret raw byte storage as typed objects unless alignment and lifetime are unquestionably valid.
      - Prefer safe byte loading patterns over reinterpret_cast.
      - When decoding binary data, copy or assemble bytes into the destination representation explicitly.
      - Do not assume that data loaded from files or arbitrary buffers is suitably aligned for typed reads.
    guidance:
      - Plausible-looking low-level code can still be undefined behavior.
      - If alignment or aliasing is in doubt, the direct cast is the wrong answer.

  nodiscard_and_results:
    headline:
      Important results must not be easy to ignore.
    rules:
      - Use [[nodiscard]] on functions and types whose return values signal status, failure, or required follow-up action.
      - Do not discard error-bearing or validity-bearing return values casually.
      - Prefer APIs that force the caller to confront failure instead of silently continuing.
    guidance:
      - If a result matters for correctness, make ignoring it harder.
      - Compiler help is better than hoping reviewers notice a dropped return value.

  headers_and_includes:
    headline:
      Keep dependency surfaces small and visible.
    rules:
      - Include only what you use.
      - Prefer forward declarations where appropriate and safe.
      - Do not let headers accumulate unrelated utilities.
      - Keep implementation details out of public headers when possible.
      - Minimize transitive include burden.
    guidance:
      - Headers should describe interfaces, not become dumping grounds.

  standard_library:
    headline:
      Prefer standard library facilities before custom reinvention.
    rules:
      - Reach for the C++ standard library first.
      - Do not introduce custom containers, strings, smart pointers, or utility wrappers without a clear repository-specific need.
      - Use familiar standard types unless a different choice is justified by evidence.
    guidance:
      - Boring standard facilities are easier to review, port, and test than custom "better" versions.

  performance:
    headline:
      Optimize late and locally.
    rules:
      - Do not pre-emptively complicate code for theoretical speed.
      - Keep hot-path optimization isolated to the code that actually needs it.
      - Do not infect broad APIs with low-level performance tricks unless profiling justifies it.
      - Prefer the simple version first, then optimize with evidence.
    guidance:
      - Performance work should be surgical, not ideological.
      - The codebase should not become harder to trust in exchange for hypothetical wins.

portability_rules:
  - Code must not depend on one compiler's extensions unless explicitly isolated and justified.
  - Do not assume platform-specific behavior without an abstraction boundary.
  - Prefer portable standard C++23 facilities first.
  - If non-portable code is truly required, isolate it behind a narrow boundary and document why.
  - Do not use vendor-specific behavior as a convenience shortcut in ordinary code.

author_operating_rules:
  - First inspect nearby code and mirror established local patterns.
  - Do not introduce a second pattern for a problem the repository already solved.
  - Do not widen the repository subset during implementation.
  - Do not infer that a more advanced C++ feature is preferred just because it is available.
  - When multiple solutions are valid, choose the one with the clearest ownership.
  - When multiple solutions are valid, choose the one with the least hidden control flow.
  - When multiple solutions are valid, choose the one with the most obvious runtime cost.
  - If a construct makes review harder, refactoring riskier, or tests less trustworthy, avoid it.
  - If unsure, choose the simpler and more explicit construct.
  - Do not import patterns from random online C++ examples unless the repository already uses them.
  - Do not solve a local problem by increasing global complexity.
  - Do not mistake "idiomatic C++" for "good fit for this repository."

decision_rules:
  - Prefer code that is obviously correct over code that is merely idiomatic.
  - Prefer local clarity over reusable cleverness.
  - Prefer one boring pattern used consistently over several elegant patterns mixed together.
  - Prefer explicit data flow over abstraction layers.
  - Prefer code that fails loudly in tests over code that can be secretly wrong.
  - Prefer designs that make future refactors smaller, not more theatrical.
  - If the clever solution and the boring solution are both valid, choose the boring solution.

examples:
  ownership_examples:
    acceptable:
      - Returning std::unique_ptr<T> from a factory when heap ownership is required.
      - Passing T& when the callee requires a valid existing object and does not own it.
      - Passing T* when null is meaningful and ownership is not transferred.
    avoid:
      - Returning raw owning pointers.
      - Storing borrowed pointers without a clear lifetime guarantee.
      - Using std::shared_ptr<T> because ownership was not designed clearly.

  error_handling_examples:
    acceptable:
      - Returning std::optional<T> when a lookup may fail to find a value.
      - Returning a status or result object when the caller needs error details.
      - Using assertions for invariants that indicate programmer error.
    avoid:
      - Throwing exceptions.
      - Returning success while hiding failure in logs or mutable globals.
      - Using assertions for expected runtime input problems.

  composition_examples:
    acceptable:
      - Building larger behavior by combining small types with clear responsibilities.
      - Using interfaces only when runtime polymorphism is genuinely needed.
      - Marking concrete implementation classes final when extension is not intended.
    avoid:
      - Deep inheritance chains.
      - Base classes created for speculative reuse.
      - Virtual-by-default class design.

  template_examples:
    acceptable:
      - A small generic helper where the type parameter directly removes obvious duplication.
      - A simple container or algorithm adapter with plain, readable constraints.
    avoid:
      - Template machinery that obscures behavior more than it removes duplication.
      - Type-trait indirection layers for ordinary business logic.
      - Compile-time cleverness that makes diagnostics and refactors worse.

review_checklist:
  ask_before_merging:
    - Is ownership obvious from local code?
    - Is control flow obvious from local code?
    - Is failure handling explicit?
    - Is runtime cost reasonably visible?
    - Does this follow existing repository patterns?
    - Did this change introduce a new pattern without a strong reason?
    - Would a future maintainer understand this without knowing obscure C++ tricks?
    - Would the simplest valid version of this code look substantially different?
  reject_if:
    - The change introduces hidden ownership.
    - The change introduces hidden control flow.
    - The change introduces unnecessary abstraction.
    - The change uses advanced C++ features without a repository-local need.
    - The change makes testing, review, or refactoring meaningfully harder.

closing:
  message:
    This repository uses a narrow C++ subset on purpose.
    The goal is not to show how much C++ you know.
    The goal is to produce portable, explicit, testable code that is easy to review and hard to accidentally corrupt.
