# Historical SDSL-V shader policy

Historical shader evidence is retained under
`internal/prometheus/DevelopmentReport/artifacts/SDSL_V_ORIGINAL_SPIRV_REWRITE/`.
Those files are audit inputs only and must not be added to the production
registry. This directory is a policy marker rather than a second copy of that
evidence: keeping the artifacts beside their M34/M35 audit reports preserves
their provenance without creating duplicate source authority.

Some older generated headers remain production build inputs for stable legacy
registry assets. They are therefore **not** historical/audit-only, even when
their manifest provenance says `historical generated`.
