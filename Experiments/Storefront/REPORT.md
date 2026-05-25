# Storefront Experiment — Short Paper

## Abstract
The Storefront experiment evaluated whether Machina UI could author and maintain a realistic storefront-style surface, not just control-panel interfaces. Across milestones M0 through M7, the experiment validated that explicit composition, deterministic state transitions, and table-first representation can scale to website-like pages without introducing new runtime subsystems. The close-out result is a stable authoring model for storefront-class UI source.

## Motivation
After SignalLab, Storefront was selected to stress a different UI shape: a content-heavy, website-style page with repeated cards, filters, sections, and navigation-like interactions. The objective was to pressure Machina UI on four fronts at once: layout expression, interaction wiring, data modeling, and source organization. It was explicitly intended to move beyond control-panel-only surfaces and test whether the same model remained readable under broader page structure.

## Method
The experiment progressed incrementally through focused milestones:

- **M0:** establish basic storefront viability with existing primitives.
- **M1:** introduce typed coordinates (`px` / `ui`) for layout domain clarity.
- **M2:** consolidate repeated interaction wiring into canonical event families.
- **M3:** separate composition concerns from behavior concerns.
- **M4:** make slot-level data explicit.
- **M5:** represent catalog/grid truth in table-shaped data.
- **M6:** use dispatch tables for simple exact-key transitions.
- **M6a:** express exact-key pure mappings as tables rather than branching.
- **M7:** stabilize top-level source organization via a lightweight parts list.

Each milestone was constrained to authoring and representation changes unless runtime change was strictly necessary.

## Key Findings
- Machina UI layout generalized beyond control panels into storefront-like page structure.
- Typed coordinates (`px` / `ui`) materially improved layout readability and edit safety.
- String-based event transport remained acceptable once canonical event families were adopted.
- Composition and behavior should be separated, but kept nearby in the same source unit.
- Table-shaped truth should be represented as tables.
- Storefront catalogs are naturally table data.
- Storefront placement is naturally grid-shaped and benefits from explicit placement tables.
- Simple exact-key transitions should use dispatch tables.
- Exact-key pure mappings should be data, not branching logic.
- Larger Machina UI files benefit from a lightweight top-level parts list.

## Canonical Machina UI Authoring Structure
The experiment converged on a consistent top-level structure for larger surfaces:

- **Data:** canonical table-shaped facts (catalogs, labels, static maps).
- **Placement:** explicit geometry/slot/grid placement tables.
- **Dispatch:** exact-key transition and resolver tables.
- **Behavior:** procedural update logic, family matching, and derived transitions.
- **Composition:** UI builders that assemble sections/cards and emit events.
- **Surface:** final assembled `View` returned from composition.

This structure keeps representation explicit while preserving deterministic behavior and local editability.

## Boundaries / Non-Conclusions
The Storefront experiment did **not** conclude that Machina UI needs:

- a runtime router,
- a typed event runtime,
- automatic layout or grid inference,
- a framework DSL.

It also did not support moving procedural logic into data indiscriminately: procedural/dynamic logic should remain code, and Octomata should remain the mechanism for real behavioral progression rather than trivial keyed mappings.

## Conclusion
From M0 to M7, Storefront provided sufficient evidence to stabilize a canonical Machina UI authoring model for storefront-like pages: explicit data and placement, table-oriented dispatch for exact-key cases, procedural behavior where needed, and clear composition-to-surface assembly.

---

# Storefront M0 Report

## Fictional brand/concept chosen

The storefront uses **Northline Supply Co.**, a believable small DTC brand focused on modular desk gear and specialty stationery for focused work setups. Product copy stays grounded in practical desk hardware (riser, cable anchors, timer, notebook, ruler, tray) so the page feels like a real niche store rather than placeholders.

## Page structure built

The M0 page is composed as a single headless storefront surface with these major regions:

1. **Header/top nav**
   - Brand mark text
   - Top-level nav actions (Shop/Bundles/Journal/Support)
   - Search stub text
   - Cart count status
2. **Announcement + hero**
   - Announcement strip for operational/news context
   - Hero headline + short brand statement
   - Three featured callout blocks
3. **Category/filter rail**
   - Category chips (All/Desk/Stationery/Cable)
   - Toggle chips (New/In stock/Bestsellers)
   - Sort chips (Featured/Price/Newest)
4. **Product listing area**
   - Product area header
   - Featured collection tab strip
   - Repeated product card grid (6 cards)
   - Category-aware card slots (cards collapse to filtered summary when hidden)
5. **Supporting sections**
   - Benefit/review/editorial support band
   - Footer with grouped link columns + selected product status

## Did current UI system handle storefront shape cleanly?

Yes. The same hybrid approach used in control-oriented experiments carried over cleanly:

- AnchoredBox for macro page regions
- Row/Column for local flow inside each region
- AbsoluteBox for predictable card-grid placement
- local coordinate constants in `View` for hygiene

No runtime UI changes were required.

## Did repeated product-card composition feel natural or painful?

Mostly natural at M0 scale. `ProductCard` and `ProductCardSlot` helpers made repeated cards manageable and kept per-card variation to data + event token wiring. The awkward part is explicit event function volume (`SelectX`, `AddX`) because string token families are manual.

## Did hybrid layout still feel correct outside dashboard/control-panel shapes?

Yes. Hybrid stayed the right default for a storefront page:

- anchored macro boxes make region-level composition easy to scan
- row/column keeps local text and button groups readable
- absolute placement is useful for dense, repeatable card rows

The model generalized without introducing new layout primitives.

## What helper patterns were most useful?

Most useful author-level helpers:

- `NavItem`
- `FilterChip`
- `SectionHeader`
- `FeatureBlock`
- `PriceBadge`
- `ProductCard`
- `ProductCardSlot`
- `FooterColumn`

These emerged naturally and reduced repeated UI assembly without adding runtime features.

## Single biggest pain point

The single biggest pain point was **manual string event proliferation for repeated cards** (inspect/add tokens per card). Layout composition itself was stable; event-token plumbing is what felt most repetitive.

## Can Oct UI generalize beyond control panels?

For this M0 probe: **yes, plausibly**. A realistic storefront-like composition was achievable with the current UI surface and style constraints, and the page retained deterministic rendering and explicit state behavior.

## Direct answers to evaluation questions

1. **Website-style structure cleanly handled?** Yes.
2. **Repeated cards natural or painful?** Natural overall; repetitive token wiring is the friction point.
3. **Hybrid layout outside dashboards?** Yes, still a good default.
4. **Easy parts?** Macro region composition, repeated card helper reuse, footer/support strips.
5. **Awkward parts?** Explicit per-card token boilerplate.
6. **LLM-friendly local edits?** Yes; coordinate constants + helper-based sections localize edits.
7. **Reusable patterns emerged?** Nav/filter/card/feature/footer helpers emerged clearly.
8. **Missing primitives?** No urgent layout primitive missing; biggest gap is ergonomics for repeated event-token families.

## Recommendation

**Continue broader website-style UI experiments** with the current primitives, while targeting one focused ergonomics pass on repeated interaction wiring (author-level conventions first, runtime changes only if multiple experiments repeat the same pain).

---

# Storefront M1 Report (Typed Coordinates Probe)

## What changed in M1

M1 keeps the same Northline storefront structure as M0 (header/nav, announcement, hero, filter rail, product grid, support band, footer), but migrates layout coordinates to explicit typed domains:

- `ui` for macro anchored regions (`HeaderTop`, `HeroBottom`, `FooterTop`, etc.)
- `px` for dense absolute card placement (`CardX0`, `CardY1`, `CardWidth`, `CardHeight`)

`AbsoluteBox` and `AnchoredBox` signatures now encode those domains directly, so the page source makes coordinate intent visible at call sites.

## Did `px` / `ui` improve readability?

Yes, materially.

- Region constants now read as normalized anchors by construction (`0.14 ui`), which makes the page flow easier to scan.
- Card constants now read as fixed-position layout values by construction (`570 px`), which clarifies local dense-grid editing.
- Mixed-domain mistakes are no longer “just numbers”; they are type errors.

For this storefront shape, typed coordinates reduced mental bookkeeping during edits because the number itself and the domain are co-located in the source.

## Did box-native layout become more explicit and safer?

Yes.

- `AnchoredBox` takes only `ui`.
- `AbsoluteBox` takes only `px`.
- Cross-space misuse fails at type-check time rather than relying on author discipline.

This made the hybrid layout contract explicit without introducing any new layout primitive.

## Did local edits feel easier?

Yes, especially in four edit zones:

1. **Card grid spacing** — card offsets/sizes are all `px`, so spacing tweaks stay in a single unit vocabulary.
2. **Filter rail width and macro section boundaries** — anchors remain `ui`, making proportional region edits straightforward.
3. **Support/footer spacing** — vertical strip edits remain in `ui`, matching page-band intent.
4. **Hero/header strip bounds** — top/bottom anchors are typed, so accidental pixel edits are prevented.

## Did repeated card/grid layout benefit?

Yes. Repeated absolute card placement became clearer because every coordinate and size constant is visibly in `px`, and the same typed constants are reused across all card slots.

## Is hybrid layout still the right default?

Yes. M1 still validates:

- anchored macro regions for page sections,
- row/column for local grouping,
- absolute placement for dense repeated cards.

Typed coordinates strengthened this model rather than replacing it.

## Biggest friction point after M1

The biggest friction point is still **event-token wiring verbosity** for repeated product interactions. Coordinates are now significantly clearer; repetitive `SelectX`/`AddX` token plumbing is still the dominant authoring friction.

## Recommendation after M1

Adopt typed coordinates (`px`, `ui`) as a real Machina UI feature in their current narrow form.

- Keep scope limited to coordinate domain separation.
- Do not expand into CSS-style unit families in the next pass.
- Consider later hardening (such as optional `ui` range policy) only if experiments show concrete value.

---

# Storefront M2 Report (Event Family Probe)

## What changed in M2

M2 keeps M1’s layout model intact (AnchoredBox/AbsoluteBox + Row/Column + typed coordinates), but replaces per-product interaction token helpers with canonical event families:

- `InspectEvent(productId)`
- `AddToCartEvent(productId)`

Product identity is now centralized in `CatalogProductIds()` plus `ProductById(...)`, and repeated card rendering plus event matching both resolve against that same source.

## Bespoke event-token patterns removed

Removed the bespoke per-product interaction helpers from M1, including:

- `SelectHarborBoardEvent`, `SelectLinenNotebookEvent`, `SelectCableDockEvent`, ...
- `AddHarborBoardEvent`, `AddLinenNotebookEvent`, `AddCableDockEvent`, ...

This eliminated one event constructor per product/action combination and replaced it with one constructor per action family.

## Event family + matching pattern introduced

M2 uses explicit string families with shared prefixes:

- inspect family: `storefront.inspect.<product-id>`
- add-to-cart family: `storefront.cart.add.<product-id>`

Matching is handled by a small author-level helper:

- `MatchCatalogEvent(event, prefix) -> productId | ""`

`Update(...)` remains explicit and deterministic:

1. handle nav/filter/sort/featured events directly as before
2. resolve inspect family against catalog and set `SelectedProduct`
3. resolve add family against catalog and increment `CartCount`
4. otherwise return unchanged model

No runtime transport changes were required; events remain plain strings.

## Did product identity become more centralized?

Yes. M1 spread identity across:

- dedicated product constructor functions
- dedicated per-product inspect/add event helpers
- mirrored update branches

M2 centralizes identity in `CatalogProductIds()` and `ProductById(...)`, and uses `product.Id` as the shared key for:

- repeated card wiring (`InspectEvent(product.Id)`, `AddToCartEvent(product.Id)`)
- update matching (`MatchCatalogEvent(...)`)
- selected-product state

## Did update branching become materially cleaner?

Yes. The repeated 12-branch per-product event ladder (6 inspect + 6 add) is replaced by two family resolution steps. Control flow is still local and readable in `Update(...)`, but significantly less repetitive.

## Is string transport still acceptable after this pass?

For this storefront probe: yes.

The main friction from M0/M1 was not string transport alone; it was missing author-level conventions on top of string transport. With canonical families + catalog identity, the repeated wiring cost drops substantially while preserving explicitness.

## Remaining interaction ergonomics pain

The next pain point is manual catalog indexing in repeated absolute placements and in catalog matching checks. This is still explicit and deterministic, but somewhat verbose because this probe intentionally avoids introducing a broader abstraction/framework layer.

## Is a deeper event-system change justified yet?

Not yet for this scenario.

M2 suggests that a canonical authoring pattern on top of string events resolves most of the prior pain without runtime redesign. A typed event runtime should be considered only if future experiments show substantial additional failure modes that families + centralized identity cannot address.

---

# Storefront M3 Report (Composition vs Behavior Separation Probe)

## What changed in M3

M3 keeps M2 behavior and runtime transport intact, but reorganizes authoring into adjacent layers inside one local source file (`M3/storefront_m3.oct`) to split concerns without fragmenting edits:

1. **Data/catalog layer**
   - `CatalogProductIds`, `ProductById`, `Init`
2. **Event family layer**
   - canonical event constructors + prefixes (`InspectEvent`, `AddToCartEvent`, nav/filter/sort/featured tokens)
3. **Behavior layer**
   - event resolution and transitions (`MatchCatalogEvent`, `Update`, state writers like `WithNav`)
4. **Composition layer**
   - visual section helpers (`HeaderBar`, `FilterRail`, `ProductCard`, `Footer`, etc.)
5. **Surface assembly layer**
   - `View` composes page regions and card placement only

No runtime redesign, no typed-event system, and no new routing framework were introduced.

## How composition and behavior were separated

Composition helpers now answer:
- what appears on the screen,
- where it is placed,
- which event family each interactive element emits.

Behavior helpers now answer:
- what each event means,
- how event families resolve against catalog identity,
- which model fields change and which remain unchanged.

`ProductCard` still emits `InspectEvent(product.Id)` / `AddToCartEvent(product.Id)` at the UI edge, while semantic meaning stays in `MatchCatalogEvent` + `Update`.

## Did composition readability improve?

Yes. The composition block can be read as a storefront sketch:
- page sections and card slots are clustered in the view/composition region,
- product card content and emitted families are visible without scanning transition logic,
- reusable section helpers remain near the final `View`, preserving quick edit loops.

This reduced the “everything soup” feeling compared to mixing transition branches and section composition in one continuous region.

## Did behavior readability improve?

Yes. `Update` and its behavior helpers are grouped with event family/data matching code, so event interpretation is readable as one flow:
1. direct nav/filter/sort/featured transitions,
2. inspect family resolution,
3. add-to-cart family resolution,
4. explicit unchanged fallback.

This makes state transition simulation easier without reading layout code.

## Was locality preserved?

Yes, and still acceptable for component-style editing.

M3 deliberately keeps the split in one nearby file so common edits remain local:
- layout tweak: mostly composition/view region,
- card interaction tweak: composition emission + adjacent behavior section,
- catalog interaction tweak: nearby data + event matching helpers.

Locality is slightly more structured than M2, not more fragmented.

## Direct answers to required M3 questions

1. **Does separating composition from behavior materially improve readability?**
   - Yes; both sides are easier to scan independently.
2. **Is the storefront easier to sketch mentally from composition code?**
   - Yes; section/card structure is more legible without behavioral noise.
3. **Is update logic easier to reason about from behavior code?**
   - Yes; event meaning and transitions are clustered together.
4. **Is locality preserved?**
   - Yes; split remains adjacent and does not recreate disconnected HTML/CSS/JS fragmentation.
5. **Does this reduce JSX-style “everything soup”?**
   - Yes; event emission remains local, but interpretation is moved out of composition.
6. **New biggest friction point after this split?**
   - Manual repeated card slot placement/indexing in `View` remains verbose.
7. **Need stronger conventions or deeper support later?**
   - Stronger authoring conventions now (layer ordering + naming) look sufficient; deeper event/runtime changes are not yet justified by this probe.

---

# Storefront M4 Report (Slot Data Probe)

## What changed in M4

M4 keeps M3’s data/event/behavior/composition separation and runtime model, and introduces an explicit slot data layer for repeated product-card placement:

- `CardSlotTable` record with parallel arrays:
  - `SlotIds` (inspectable identity like `r0c0`)
  - `ProductIds` (catalog keys)
  - explicit geometry arrays (`X`, `Y`, `Width`, `Height`)
- `CardSlots() -> CardSlotTable` as the single explicit slot map.
- `CardSlotPlacements(model, slots) -> UI[]` to convert slot data into concrete `Place(AbsoluteBox(...), ...)` nodes.

No auto-layout, inferred grid, or runtime repeater was added.

## Boilerplate reduced vs M3

M3 repeated six near-identical placement lines in `View`, each manually pairing:

- `AbsoluteBox(x, y, width, height)`
- `ProductById(productIds[i])`
- `ProductCardSlot(...)`

M4 removes that repeated per-line indexing/wiring from `View` by:

1. centralizing slot geometry + product mapping in `CardSlots()`,
2. using one small author-level loop (`CardSlotPlacements`) for placement expansion.

The repetition is reduced materially, and moved into a data table plus one deterministic expansion helper.

## Did geometry remain explicit?

Yes.

Every slot still carries concrete coordinates and size in source:
- `X`, `Y`, `Width`, `Height` values are listed explicitly in `CardSlotTable`.
- There is no “place N cards somehow” behavior.
- Grid placement remains author-declared and concrete.

## Did sketchability hold?

Yes, with a tradeoff that still favors readability:

- M3 sketchability came from six explicit placement calls in `View`.
- M4 sketchability comes from a compact slot table (`CardSlots`) where each index is inspectable (`SlotIds[i]`, `ProductIds[i]`, explicit geometry arrays).

A reviewer can still quickly answer:
- how many cards (6),
- where they go (explicit px coordinates),
- which product is in each slot (ProductId per row).

## Locality impact

Locality remains acceptable and in some paths improved:

- Slot placement edits now happen in one local table (`CardSlots`) rather than scattered placement lines.
- Composition/behavior separation from M3 remains intact.
- `View` is shorter and easier to scan for macro regions.

The only mild cost is one additional indirection (`CardSlots` + placement helper), but it stays adjacent and explicit.

## Direct answers to required M4 questions

1. **Does slot data materially reduce repeated indexing/placement boilerplate?**
   - Yes; repeated placement/index lines are replaced by one slot table + one expansion helper.
2. **Does the page remain mentally sketchable from source?**
   - Yes; slot rows preserve explicit geometry and product mapping.
3. **Are arrays/records helping, or just moving repetition elsewhere?**
   - Helping; repetition is consolidated into a clearer data structure, not hidden in layout inference.
4. **Is the code easier to scan?**
   - Yes; `View` now emphasizes macro structure while slot specifics live in one compact list.
5. **Does locality remain acceptable?**
   - Yes; slot edits are centralized, and behavior/composition layering remains nearby.
6. **Does this suggest a canonical explicit-slot-data pattern?**
   - Yes; for dense repeated placements, a slot table (`SlotIds` + `ProductIds` + explicit coordinate arrays) is a strong author-level default.
7. **Next biggest pain point after this pass?**
   - Repetition inside catalog product declarations and static section copy (not slot placement) is now the dominant authoring bulk.

## Recommendation after M4

Adopt **explicit slot data structures** (`SlotId` + `ProductId` + explicit geometry data) as a canonical Machina UI authoring pattern for repeated fixed-card layouts.

- Keep it author-level only.
- Preserve explicit geometry.
- Avoid introducing inferred layout/runtime repeaters.

---

# Storefront M5 Report (Catalog/Grid Data Probe)

## What changed in M5

M5 preserves M4’s runtime, event families, and composition-vs-behavior structure, and changes only author-level data modeling:

- Catalog moved from ad hoc branch-based construction to an explicit table:
  - `CatalogTable() -> ProductCatalog`
- Product lookup now scans the catalog table:
  - `ProductById(productId)` loops `CatalogTable()` rows instead of a hand-written `if` ladder
- Product placement moved from flattened slot lists to explicit row/column grid data:
  - `ProductGrid() -> ProductPlacementGrid`
  - `ProductIds: String[]` + `RowStarts: Int[]` + `RowLengths: Int[]` for explicit row/column assignment
  - `ColumnX`, `RowY`, `CardWidth`, `CardHeight` for explicit deterministic geometry
- Card expansion remains author-level and deterministic:
  - `CardSlotPlacements(model, grid)` expands row/column + product id into concrete `Place(AbsoluteBox(...), ...)` nodes

No auto-grid, runtime repeater, hidden layout inference, or data-binding system was introduced.

## Catalog data structure introduced

M5 uses `ProductCatalog` (parallel field arrays) as the canonical catalog table. Each row index carries:

- `Id`
- `Title`
- `Subtitle`
- `Price`
- `Badge`
- `Category`

This makes the product declaration read like plain table data instead of control-flow code.

## Placement/grid data structure introduced

M5 uses an explicit placement record:

- `ProductIds: String[]` with explicit row boundaries (`RowStarts`, `RowLengths`) for row/column product assignment
- `ColumnX: Float<px>[]`
- `RowY: Float<px>[]`
- `CardWidth`, `CardHeight`

This keeps arrangement inspectable as:
- row 0: harbor-board, linen-notebook, cable-dock
- row 1: focus-timer, cork-tray, steel-ruler

while preserving explicit pixel geometry.

## Did product data read more naturally as data?

Yes. The catalog is now visibly table-shaped data (`ProductCatalog`) with one index-aligned row per product, which is easier to scan and review than an `if productId == ...` chain.

## Did page arrangement become easier to sketch mentally?

Yes. Arrangement is now directly row/column-shaped in source (flattened `ProductIds` plus explicit row boundaries) and still tied to explicit `ColumnX`/`RowY` geometry, so both logical order and concrete placement remain clear.

## Repetition reduced

M5 materially removes:

- branch-heavy product lookup/definition ladder in `ProductById`
- manual per-index event matching branches in `MatchCatalogEvent`
- flattened product-assignment repetition by replacing one-dimensional slot product mapping with a row/column table

Remaining repetition is mostly static copy/state-field boilerplate, not catalog/grid authoring.

## Direct answers to required M5 questions

1. **Does catalog data become clearer when modeled explicitly as data?**
   - Yes; a table-shaped `ProductCatalog` is clearer and easier to scan than branch code.
2. **Does a product-id grid/table make arrangement more mentally sketchable?**
   - Yes; row/column IDs map directly to visible storefront rows.
3. **Does this reduce repeated lookup/definition boilerplate materially?**
   - Yes; lookup and event matching both moved from hand-expanded branches to small loops over explicit data.
4. **Is the page still easy to reason about from source?**
   - Yes; macro layout and card geometry remain explicit/deterministic.
5. **Does this improve or hurt locality?**
   - Improves locality for catalog and placement edits by centralizing each in one table-shaped block.
6. **Is table/grid the right canonical direction for storefront-like UI data?**
   - Yes, for fixed-card catalog pages where inspectability and determinism are required.
7. **What is the next biggest pain point?**
   - Repetitive state-copy/update boilerplate (`WithX` patterns and repeated state field carry-over) is now the dominant authoring friction.

## Recommendation after M5

Adopt **catalog table + placement grid tables** as a canonical Machina UI authoring style for storefront-like screens, with these guardrails:

- keep placement explicit and deterministic (no inferred layout)
- keep expansion helpers small and obvious
- preserve composition/behavior separation from M3/M4

This direction reduces authoring bulk and keeps sketchability intact.

---

# Storefront M6 Report (Dispatch Table Probe)

## What changed in M6

M6 preserves the M3–M5 authoring architecture (data tables, family events, explicit composition), but replaces direct exact-key event ladders with explicit dispatch tables for simple deterministic mappings:

- nav event key -> selected nav value
- category event key -> selected category value
- sort event key -> sort value
- featured event key -> featured tab label

This is implemented with a small `EventValueDispatch` record plus family-scoped table constructors and a deterministic resolver:

- `NavDispatchTable`, `CategoryDispatchTable`, `SortDispatchTable`, `FeaturedDispatchTable`
- `ResolveDispatch(event, table) -> value | ""`

`Update(...)` now performs table resolution for table-shaped transitions, while keeping procedural logic in code for:

- toggle filters (`new`, `in stock`, `best seller`)
- catalog event-family matching (`inspect`, `add-to-cart`)

No runtime event model changes were introduced.

## Which ladders were replaced

Replaced keyed `if` ladders for:

- nav selection
- category selection
- sort selection
- featured-tab selection

Kept as explicit code (not forced into tables):

- filter toggles (boolean inversion logic)
- inspect/add family resolution and cart increment behavior

## Trivial `With*` helper removal

M6 removes these micro update helpers:

- `WithNav`
- `WithCategory`
- `WithCartCount`

They are replaced by direct `model with { ... }` updates at the point of dispatch resolution.

## Answers to required M6 questions

1. **Are keyed ladders table-shaped?**
   Yes, for nav/category/sort/featured they were direct key->value mappings.

2. **Boilerplate reduction?**
   Yes. Repeated ladders became compact table declarations plus one resolver path per family.

3. **Reasoning clarity?**
   Improved for simple mappings: each family is now a scan-friendly event/value table.

4. **Locality impact?**
   Improved slightly: table declarations, resolver, and `Update` remain adjacent in behavior scope.

5. **Which paths belong in tables vs code?**
   - Tables: exact-key to exact-value transitions.
   - Code: toggles and catalog family matching (procedural/derived logic).

6. **Can trivial helpers be removed cleanly?**
   Yes, for these cases direct `with` updates were clearer and removed indirection.

7. **Next biggest pain point after direct dispatch tables?**
   The remaining verbosity is mostly repeated catalog-driven presentation wiring (card-slot assembly/index traversal), not event dispatch itself.

## Recommendation

Adopt this dispatch-table pattern as a canonical behavior-authoring style for **simple keyed UI transitions** in Machina UI code:

- keep it narrow to direct key->value mappings,
- keep resolver deterministic and local,
- keep procedural logic outside tables.

Do **not** generalize this into a runtime router, typed event system, or generic dispatch DSL at this stage.

---

# Storefront M6a Report (Pure Mapping Cleanup)

## What changed in M6a

M6a is a narrow cleanup on top of M6. It keeps M6 structure and behavior intact, while converting the remaining obvious exact-key pure value ladders into table-shaped data + tiny deterministic lookup code.

Converted in M6a:

- `CategoryLabel(...)` from `if key == ...` branches to `CategoryLabelMapping()` + `ResolveLabelMapping(...)`
- `SortLabel(...)` from `if key == ...` branches to `SortLabelMapping()` + `ResolveLabelMapping(...)`

Not converted in M6a:

- dynamic event families (`InspectEvent(productId)`, `AddToCartEvent(productId)`) because they are procedural string construction, not exact-key mapping tables
- boolean toggle behavior in `Update(...)` because it is transition logic (`not model.X`), not pure mapping

## Event constant families: kept or changed?

The nav/category/sort/featured event constant families were **kept as functions** in M6a.

Reasoning:

- they remain readable at the UI emission surface (`Button(..., NavShopEvent(), ...)`, etc.)
- they are already consumed by dispatch-table constructors, so call-site intent stays explicit
- converting them now would mostly rename/relocate constants rather than materially reducing complexity

For this narrow pass, keeping them preserved local readability without introducing additional indirection.

## Readability and locality impact

- **Readability improved** for pure labels: category/sort display mappings now read as explicit data tables rather than branch logic.
- **Locality stayed stable**: label mappings and resolver are kept adjacent to the behavior/composition helpers that use them.
- no generic mapping framework or cross-file abstraction was introduced.

## Logic vs data boundary after M6a

M6a makes the boundary clearer:

- exact-key pure label mappings -> table data (`CategoryLabelMapping`, `SortLabelMapping`)
- exact-key event transitions -> dispatch tables (`NavDispatchTable`, etc.)
- dynamic/procedural behavior -> code (toggle inversion, family event construction/matching)

## Behavioral equivalence and test updates

M6a keeps storefront behavior equivalent to M6 for affected surfaces.

Added focused coverage:

- label mapping canonical values
- unknown-key fallback stability for category/sort labels

Existing interaction, dispatch, and catalog/grid determinism tests remain in place.

## Does this strengthen the “mappings are data” rule?

Yes. For exact-key pure mappings, M6a shows the rule cleanly: these are better represented as explicit data tables with tiny lookup helpers than as branch ladders.

## Next biggest authoring pain after M6a

After dispatch + pure mapping cleanup, the largest remaining friction is repeated catalog presentation wiring (card slot traversal/index plumbing), not key-mapping logic.


---

# Storefront M7 Report (Machina UI File Structure Probe)

## What changed in M7

M7 preserves M6a runtime behavior and authoring doctrine, but adds an explicit top-level source-structure declaration:

- `SourceParts` record
- `SourceStructure()` constructor

The declaration formalizes the stable file categories discovered across M3-M6a while keeping implementation code in place:

- Data
- Placement
- Dispatch
- Behavior
- Composition
- Surface

Each category lists the canonical functions that own that concern in this file (for example `CatalogTable`, `ProductGrid`, `NavDispatchTable`, `Update`, composition builders, and final `View`).

No layout runtime, event runtime, or state runtime changes were introduced.

## Categories formalized

M7 formalized this top-level parts list:

1. **Data**: catalog + pure mapping tables
2. **Placement**: explicit slot/grid and placement assembly
3. **Dispatch**: keyed event-value transition tables + resolver helpers
4. **Behavior**: procedural update and lookup logic
5. **Composition**: section/card/footer builders
6. **Surface**: final assembled `View`

## Answers to required M7 questions

1. **Does top-level declaration improve readability?**
   Yes. A reviewer can scan `SourceStructure()` first and get the file architecture immediately.

2. **Does it help reviewer/LLM navigation?**
   Yes. It gives an up-front map for “where to edit” by concern.

3. **Does it clarify where new code belongs?**
   Yes. New table truth goes to Data/Dispatch, transition logic to Behavior, UI builders to Composition, and assembly to Surface.

4. **Does it improve or worsen locality?**
   Slight net improvement. The declaration is near the top and references existing functions without moving runtime logic away from use sites.

5. **Truthful structure or ceremony?**
   Truthful in this pass. The categories match pre-existing stable concepts rather than introducing synthetic framework layers.

6. **Should Machina UI files have a canonical parts list convention?**
   Yes, for large UI sources. A lightweight `SourceStructure()` convention is useful when files mix data tables, dispatch tables, behavior, and composition.

7. **What pain remains after formalizing file shape?**
   Repeated catalog presentation wiring (index/slot traversal) remains the largest authoring friction; structure declaration does not remove that procedural repetition.

## Locality and drift check

M7 keeps declaration and implementation in the same file and avoids introducing a second runtime representation. The declaration is a navigation map only; behavior remains fully defined by existing code paths.

## Close-out recommendation

Storefront now appears sufficiently stable to propose a canonical Machina UI authoring model:

- keep M3-M6a doctrine (tables where table-shaped, dispatch tables for keyed transitions, explicit placement, procedural behavior as code)
- add an optional/lightweight top-level `SourceStructure()` parts list for multi-section UI files
- avoid expanding this into a framework DSL or metadata-heavy system

This closes the Storefront experiment with a practical source-shape convention that improves human and LLM navigation while preserving runtime simplicity.

---

# Storefront M110 Report (Canonical UI Surface Migration)

## What changed in M110

- Confirmed latest Storefront milestone is `Experiments/Storefront/M7/`.
- Migrated `storefront_m7.oct` from locally defined UI wrapper scaffolding to canonical `import UI` + `UI.*` authoring calls.
- Removed duplicated local wrappers and wrapper records (`Mount`, `UIBox`, and local wrapper helpers) from the M7 source.
- Preserved app state/update/view architecture and unit-aware layout (`Float<px>` for absolute placement; `Float<ui>` for anchored placement).

## Scope and non-goals

This was a refactor-only authoring-surface migration:

- No Storefront redesign
- No style/theme records
- No dispatch-helper rewrite
- No grid/cell layout introduction
- No Gio wiring
- No migration to the newer Go session pipeline
- No UIIR ABI/runtime changes

## Status

- Latest Storefront source now uses canonical `UI.*` construction/placement surface.
- Local wrapper duplication is removed in M7 source.
- Dispatch/style/layout-row migrations remain future work.
