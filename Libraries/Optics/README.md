# Optics

## Shelf boundary

`Optics` owns refraction, ideal lenses, interference, diffraction, aperture resolution, and optical photon relations. [`Physics`](../Physics/README.md) owns generic wave laws and shared constants; [`Signal`](../Signal/README.md) owns domain-neutral discrete signal operations. Start with the refraction and thin-lens facts in `Optics.Core.octest`.

The chapter progresses from wave speed to refraction, then lenses, interference, diffraction, aperture resolution, and photons.

`RefractiveIndex` is a refined Concept: known positive values are proved at compile time and runtime values use `AdmitRefractiveIndex`. `OpticalImage` is a record-shaped Concept because distance, magnification, and real/inverted interpretation form one domain result.

Angles are dimensionless radians in the API. Thin-lens results use the real-is-positive sign convention and the paraxial, thin-lens approximation. Two-slit fringe spacing uses the small-angle form. Diffraction functions reject non-propagating orders instead of returning NaN. This package is not a geometric ray tracer or wave-optics field solver.
