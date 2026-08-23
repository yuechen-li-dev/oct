# Geometry M0

## Purpose

`Libraries/Geometry` provides canonical geometric scalar relations for scientific and engineering code, so common formulas do not need to be rewritten by hand.

## M0 scope

Geometry M0 is intentionally small:

- Planar: distance, circle area/circumference, rectangle area, triangle area.
- Planar computational foundations: point orientation, simple-polygon area, and polygon centroid.
- Solids: cylinder volume/surface area, sphere volume/surface area, cone volume.

No computational geometry framework is included; the polygon functions are bounded mathematical foundations.

## Unit philosophy

Inputs use length (`Float<m>`) and outputs preserve derived geometric units:

- distance and circumference -> `Float<m>`
- area -> `Float<m^2>`
- volume -> `Float<m^3>`

## Validation policy

- Negative dimensions are rejected.
- Zero dimensions are allowed for M0 and produce physically meaningful zero/degenerate results.
- Point-to-point distance allows coincident points and returns zero distance.
- Polygon coordinates must have matching lengths and at least three vertices.
- Polygon centroid rejects zero signed area. Self-intersection is not diagnosed.

## Shoelace example

```oct
let xs: Float<m>[] = [0.0m, 4.0m, 4.0m, 0.0m]
let ys: Float<m>[] = [0.0m, 0.0m, 2.0m, 2.0m]
let area: Float<m^2> = PolygonArea(xs, ys)!
let center = PolygonCentroid(xs, ys)!
// area = 8 m^2, center = (2 m, 1 m)
```

## Non-goals

This package is **not** a CAD kernel, polygon framework, mesh library, collision system, or graphics transform stack.
