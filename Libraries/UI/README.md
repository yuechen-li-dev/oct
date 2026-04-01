# Machina UI (Machine Native User Interface) for Oct

Machina UI is the UI system first implemented in Oct. It is intentionally machine-native and explicit: source should read as authored layout and composition, not as hidden browser-style layout side effects.

The design goal is predictable rendering and deterministic behavior with clear boundaries between representation data, transition wiring, and procedural updates.

## Canonical Machina UI File Structure

- **Data**  
  Catalogs, label maps, and other pure data tables that represent canonical facts.

- **Placement**  
  Slot/grid tables and explicit geometry values used to place UI deterministically.

- **Dispatch**  
  Exact-key transition tables and small resolvers for direct key-to-result lookups.

- **Behavior**  
  Procedural update logic, event-family matching, and derived transitions that are not pure table lookup.

- **Composition**  
  Section builders, card builders, and local UI helpers that assemble UI and emit events.

- **Surface**  
  The final assembled `View` (or equivalent top-level UI value).

## Representation Rules

- Exact-key pure mappings should be tables.
- Exact-key simple transitions should be dispatch tables.
- Placement should remain explicit and deterministic.
- Procedural/dynamic logic should remain code.
- Composition emits events; behavior defines meaning.
