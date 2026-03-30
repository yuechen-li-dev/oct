# Geometry M0

## Purpose

`Libraries/Geometry` provides canonical geometric scalar relations for scientific and engineering code, so common formulas do not need to be rewritten by hand.

## M0 scope

Geometry M0 is intentionally small:

- Planar: distance, circle area/circumference, rectangle area, triangle area.
- Solids: cylinder volume/surface area, sphere volume/surface area, cone volume.

No computational geometry framework is included.

## Unit philosophy

Inputs use length (`Float<m>`) and outputs preserve derived geometric units:

- distance and circumference -> `Float<m>`
- area -> `Float<m^2>`
- volume -> `Float<m^3>`

## Validation policy

- Negative dimensions are rejected.
- Zero dimensions are allowed for M0 and produce physically meaningful zero/degenerate results.
- Point-to-point distance allows coincident points and returns zero distance.

## Non-goals

This package is **not** a computational geometry, graphics transform, or CAD system. It does not include intersections, polygons frameworks, meshes, collision detection, or shape hierarchies.
