# Concepts M2 — concept-modeled native capability requests

## Verdict

**SUCCESS — CONCEPT-MODELED CAPABILITIES ENFORCED.** An `[Artifact]` entry can
name a typed request provider, receive an exact host approval, invoke the
existing interpreted Octxiliary dispatcher under a compiler-owned grant, and
publish the typed result through staged `Artifact.Write*`. Without approval the
same call is denied. No Oct record can implement the Go grant interface.

## Lifecycle and model

Before M2, complete loading/type checking and deterministic artifact discovery
already preceded shared-interpreter evaluation. `Artifact.Write*` used a
compiler-owned staging broker, while the generic wrapper dispatcher rejected
every artifact call. Ordinary interpreted execution used that dispatcher and
compiled execution used generated Octxiliary client code. Sidecars were found
beside the Oct executable or through `OCT_WRAPPER_PATH`, launched once per
command, handshaken over piped stdin/stdout, reused, and closed at interpreter
shutdown.

M2 retains that path and adds four explicit layers:

1. Request: `[Artifact(Provider)]` names a zero-argument, infallible function
   returning the package-local record Concept `ArtifactCapabilityRequest` with
   `Native: NativeComputeRequest[]`. Each native atom has non-empty `Package`,
   `Wrapper`, and `Operation` strings.
2. Policy: repeatable CLI/programmatic approvals use the exact normalized
   spelling `Package:Wrapper:Operation`. Wildcards do not exist. Malformed and
   broader-than-request approvals fail.
3. Grant: the artifact orchestrator resolves atoms against loaded wrapper
   manifests and creates a Go interface value scoped to one artifact entry.
   Request records never cross this boundary as authority.
4. Enforcement: the generic dispatcher compares the full resolved manifest
   identity (package, wrapper, Oct/wire operation, family, protocol, sidecar
   command) immediately before resolving or launching the sidecar.

Discovery evaluates only the provider and ordinary helper calls before the
artifact entry. Wrapper and effectful builtin attempts are rejected. Exact
duplicates are deduplicated; normalized atoms sort lexically. Unknown packages,
wrappers, and operations fail as malformed requests. An approval with no
request fails. A requested but unapproved operation reaches the broker as a
focused denial. Availability remains separate: an approved operation with no
sidecar reports the expected command and discovery guidance.

## Native boundary and capability classification

`Native.Compute` authorizes execution of trusted, installed native code. It is
not purity and not containment. The launcher uses the resolved executable as
its only argument, leaves `Cmd.Dir` empty (therefore inheriting the caller's
working directory), leaves `Cmd.Env` nil (therefore inheriting the environment),
pipes stdin/stdout, does not attach stderr, and applies no OS filesystem,
network, process, credential, or handle sandbox. The sidecar is not passed the
Artifact output root or policy object. A malicious authorized sidecar retains
ambient authority from the operating system. WASI/OS sandboxing is a later
milestone.

`Artifact.Publish` remains the confined staged broker. Ambient reads, arbitrary
filesystem writes, process, network, environment, clock, and secure randomness
remain denied during artifact evaluation. Native authorization grants none of
those Oct APIs. In-memory hashing stays ordinary deterministic computation;
file hashing remains an ambient read. Seeded randomness remains compatible.
Encryption, secret/key-store access, and secure randomness remain separate and
default-denied; M2 adds no such APIs.

## Proof and provenance

`Language/Tooling/ConceptCapabilitiesM2/valid` uses the existing
`octxiliary-test-wrapper`. Its ordinary provider composes and duplicates one
request atom; normalization produces one atom. Tests prove default denial,
overbroad approval rejection, exact approval, shared-interpreter dispatch,
fallible typed String return, nested staged publication, unchanged rerun,
sidecar SHA-256 provenance, and absence of generated `.go` or `.exe` artifact
application files. Artifact evaluation creates zero application backend
generations, temporary runners, application compilations, or host executables;
the test explicitly prebuilds the native sidecar.

Human output records REQUEST, GRANT/DENY, and DISPATCH. JSON adds capability
records containing artifact/source, normalized identity, wrapper/wire/family,
protocol/command, requested/approved/granted/dispatched state, resolved sidecar
path, and sidecar SHA-256. Output records retain path, MIME type, byte count,
content hash, function, source, and produced/unchanged status.

The JSON measurement block records raw and normalized request atoms,
normalization microseconds, grants evaluated, approvals accepted/rejected,
dispatches, backend generations, and artifact host executables. In the decisive
fixture two composed duplicate atoms normalize to one grant and one dispatch;
backend generations and artifact host executables remain zero. The final real
CLI run on the implementation workstation reported 1,582 microseconds for
discovery/normalization and 18 ms for first publication; an immediately
preceding unchanged rerun reported 16 ms.
These whole-command timings are diagnostic, not a
benchmark; normalization is separately reported by the command.

Ordinary interpreted and compiled wrapper routes are unchanged. Compatibility
shims remain limited to the pre-existing artifact `--execution compiled` alias,
global `WriteOctagon`, and confined directory/read-after-write behavior.

## Limitations and next milestone

M2 supports only exact `Native.Compute` atoms and one package-local request
shape. It has no wildcard algebra, negation, inheritance, interactive policy,
package-qualified provider, signed manifest, sidecar version field, or sandbox.
Sidecar content is measured at dispatch but is not currently pinned in policy.
The next milestone should add an optional manifest/policy digest pin and a real
WASI or OS sandbox before making any confinement claim about native code.
