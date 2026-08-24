# Cooking Library

## Shelf boundary

`Cooking` is the canonical applied-science package for recipe scaling, ingredient conversions, brines, baker's percentages, mixtures, and practical cooling/rest calculations. [`Units`](../Units/README.md) owns conversions, and [`Thermofluids`](../Thermofluids/README.md) owns reusable heat-transfer equations; `Cooking` composes them rather than duplicating thermodynamics. Start with the preferred typed M1 surface below.

Practical cooking mathematics for oct. The things you need a calculator for
but never bother using one — so your cakes fail and your brine is wrong.

## Why this exists

Most cooking math is straightforward multiplication. But a few problems are
genuinely non-linear and widely misunderstood:

- Leavening agents (baking powder, baking soda, yeast) do **not** scale
  linearly above roughly 3×. Full linear scaling causes over-gassing,
  bitterness, and structural collapse.
- Salt perception amplifies slightly with concentration — you need a little
  less than linear at large scales.
- A cup of bread flour weighs more than a cup of cake flour. Volume-to-weight
  conversion is ingredient-specific.
- Pan size doesn't change linearly with diameter — it scales with area.

This library makes the corrections explicit and testable.

## Components

### Preferred typed M1 surface

New code should use `Cooking.Typed` functions with `Float<kg>`, `Float<m^3>`, `Float<K>`, and `Float<s>` plus `Units` records for ounce, pound, cup, fluid ounce, tablespoon, teaspoon, and Fahrenheit presentation. `IngredientQuantity` and `BrineComposition` are record-shaped Concepts that keep related physical values together.

The original scalar conversion names remain compatibility wrappers and now delegate through typed conversion paths. `CoolingRestTemperature` reuses `Thermofluids.LumpedTemperature` instead of duplicating an exponential solver.

### Unit conversion — weight

`GramsToOunces`, `OuncesToGrams`, `GramsToPounds`, `PoundsToGrams`

### Unit conversion — volume

`MlToFlOz`, `FlOzToMl`, `CupsToMl`, `MlToCups`, `TbspToMl`, `MlToTbsp`,
`TspToMl`, `MlToTsp`

All volume functions use US measurements. 1 US cup = 236.6 ml,
1 US tbsp = 14.8 ml, 1 US tsp = 4.9 ml.

### Unit conversion — temperature

`CelsiusToFahrenheit`, `FahrenheitToCelsius`, `CelsiusToGasMark`

Gas mark reference: GM1=135°C, GM2=150°C, GM3=160°C, GM4=175°C (350°F,
moderate), GM5=190°C, GM6=200°C (hot), GM7=220°C (very hot), GM8=230°C,
GM9=245°C.

### Temperature stages

`SugarStage(tempC)` returns the confectionery stage name:

| Range | Stage |
|---|---|
| 116–120°C | Soft ball (fudge, fondant) |
| 121–129°C | Firm ball (caramels) |
| 130–137°C | Hard ball (nougat) |
| 138–148°C | Soft crack (toffee) |
| 149–165°C | Hard crack (brittles) |
| 166–179°C | Clear caramel |
| 180°C+ | Dark caramel / burnt |

`BeefDoneness(tempC)` returns the doneness stage for beef and lamb.

`IsSafeTemperature(protein, tempC)` checks against food safety minimums:
poultry at 74°C, beef/pork/fish at 63°C, ground beef/eggs at 71°C.

### Volume-to-weight for common ingredients

`GramsPerCup(ingredient)` — weight of one US cup. Supported ingredients:
`all_purpose_flour` (125g), `bread_flour` (130g), `cake_flour` (100g),
`whole_wheat_flour` (120g), `cocoa_powder` (85g), `granulated_sugar` (200g),
`powdered_sugar` (120g), `brown_sugar_packed` (220g), `salt` (288g),
`butter` (227g), `honey` (340g), `milk` (244g), `water` (237g),
`rice_uncooked` (185g), `rolled_oats` (90g).

`CupsToGrams(cups, ingredient)` — convenience wrapper.

### Baker's percentages

Professional bakers express all ingredients as a percentage of flour weight.
Flour is always 100%. This makes scaling and comparing recipes reliable.

- `BakersPercentage(ingredientGrams, flourGrams)` — ingredient as % of flour
- `FromBakersPercentage(percent, flourGrams)` — weight from percentage and flour
- `TotalDoughWeight(percentages, flourGrams)` — total weight from all percentages

A 65% hydration bread has 65g water per 100g flour. A standard brioche is
~50% butter and ~60% eggs by baker's percentage.

### Recipe scaling — the centerpiece

`ScaleIngredient(amount, scaleFactor)` — linear scaling for most ingredients:
flour, sugar, butter, dairy, most spices.

`ScaleLeavening(amount, scaleFactor)` — leavening with correction:

```
≤ 3×:  full linear (exact)
> 3×:  linear up to 3×, then 75% of remaining increment
```

Example: 1 tsp baking powder scaled to 5×:
- Linear: 5 tsp
- Corrected: 3 + (2 × 0.75) = 4.5 tsp

`ScaleSalt(amount, scaleFactor)` — salt with mild correction:

```
≤ 2×:  full linear
> 2×:  linear up to 2×, then 90% of remaining increment
```

`PanAreaScaleFactor(originalDiameter, newDiameter)` — scale factor for
changing pan size. Use this result as the `scaleFactor` for ingredients.
Going from an 8-inch to a 9-inch round pan: `(9/8)² = 1.266` — you need
26.6% more of everything.

### Yield and trim loss

- `NetWeight(grossGrams, trimLossPercent)` — weight after discarding trimmings
- `GrossWeightRequired(netGrams, trimLossPercent)` — how much to buy

Example: leeks with 40% trim loss. Need 300g usable leek:
`GrossWeightRequired(300, 40)` → buy 500g.

Typical trim losses: leeks 35–40%, artichokes 60–75%, fennel 25–30%,
celery 15–20%, onions 10%.

### Brine and dilution

- `BrineSaltPercent(saltGrams, waterGrams)` — salt concentration by weight
- `SaltForBrine(waterGrams, targetPercent)` — salt needed for target concentration
- `ReductionConcentration(initialPercent, reductionPercent)` — concentration
  after reducing a liquid by evaporation

Standard brines: poultry wet brine 3–6%, deli-style cure 2–3%.
50% reduction doubles concentration. 66.7% reduction triples it.

## A complete example

Scaling a cake from an 8-inch to a 10-inch round pan:

```oct
let scaleFactor = PanAreaScaleFactor(8.0, 10.0)  // 1.5625

let flour    = ScaleIngredient(200.0, scaleFactor)   // 312.5g
let sugar    = ScaleIngredient(150.0, scaleFactor)   // 234.4g
let bakingPw = ScaleLeavening(2.0,   scaleFactor)    // 3.125 tsp (heuristic)
let salt     = ScaleSalt(1.0,        scaleFactor)    // 1.5625 tsp (heuristic)
let milk     = ScaleIngredient(120.0, scaleFactor)   // 187.5ml
```

Scaling a bread recipe 5× for a commercial batch:

```oct
let yeastLinear    = ScaleIngredient(7.0, 5.0)
let yeastHeuristic = ScaleLeavening(7.0, 5.0)
```

`ScaleLeavening` and `ScaleSalt` preserve historical library heuristics for compatibility. They are not universal food-science laws; fermentation, formulation, geometry, process time, and sensory targets can dominate. Treat them as explicit starting assumptions, not authoritative scaling rules.

## Test coverage

Compatibility and typed contracts cover all functions, round-trip properties, the leavening
correction behaviour at multiple scale factors, pan area scaling, brine
concentration inverse properties, and a full end-to-end baking scenario.
