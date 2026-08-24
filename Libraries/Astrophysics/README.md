# Astrophysics

`Astrophysics` owns orbital, stellar-radiation, and astronomical-distance applications. [`Physics`](../Physics/README.md) owns gravitational and radiation constants, [`Units`](../Units/README.md) owns astronomical unit conversions, and [`Simulation`](../Simulation/README.md) can execute repeated normalized scenarios. Start with the circular-orbit example below.

## Orbits

Newtonian two-body relations connect gravitational parameter, circular speed, period, escape speed, specific orbital energy, vis-viva, and the Hill approximation. `CircularOrbit` is a record-shaped Concept so radius, speed, period, and specific energy travel as one readable scientific result. Orbital radius is measured from the central body's center, not from its surface; `SpecificEnergy` is J/kg, not total energy.

```oct
import Astrophysics
import Units

let earthMass = Units.EarthMassToKilogram(Units.EarthMass { Value: 1.0 })
let earthMeanRadius = 6371000.0m
let orbit = Astrophysics.CircularOrbitAt(earthMass, earthMeanRadius + 400000.0m)?
```

External callers qualify package functions and propagate domain failures with `?`; package-local tests use `!` only for known-good cases.

## Radiation and distance

Blackbody luminosity, inverse-square radiant flux, Wien displacement, distance modulus, parallax, and wavelength redshift form the second chapter. Existing `Units.Astronomical` records supply AU, parsec, solar/planetary masses, solar radius, and solar luminosity rather than re-declaring them. `DistanceObservationAt` pairs a parsec distance with its derived modulus.

Orbit functions assume Newtonian point masses (or spherical bodies outside their radius), negligible test-body mass where implied, and no perturbations or relativity. Hill radius is a circular restricted-three-body approximation. Blackbody relations use an ideal uniform photosphere. This is not an N-body integrator or cosmology framework.
