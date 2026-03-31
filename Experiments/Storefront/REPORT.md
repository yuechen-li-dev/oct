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
