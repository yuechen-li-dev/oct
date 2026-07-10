# Prometheus shader and implementation ID policy

Shader asset IDs and compute implementation IDs are separate stable namespaces.
Their table position and generation order are not identity. Existing public SGEMM
implementation IDs 1 through 11 retain their numeric values in R3. A removed
public ID must be represented as a tombstone before it can be reused; no
generator is permitted to allocate or renumber IDs. Registry validation rejects
duplicate IDs, missing shader references, non-compute shader references, empty
SPIR-V, malformed byte sizes, and selector-eligible nondispatchable entries.

Selector and numerical policy do not belong to the ID table. In particular,
request-specific FP16 conservative quantization eligibility remains judgment
logic, not a static descriptor property.
