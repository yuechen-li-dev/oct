# Mechanics

`Mechanics` is the engineering-analysis side of the science shelf. Foundational laws such as `F = m*a`, impulse, and point-particle kinematics live in `Physics`; this package applies those laws to stresses, shafts, fatigue, continuum models, beams, columns, and pressure vessels.

## M1 structural chapter

- section properties for rectangles
- axial strain, Hooke law, and thermal expansion
- uniform-bar axial deformation
- centered-load simply supported beams
- end-loaded cantilevers
- Euler ideal-column buckling
- thin-wall cylindrical pressure-vessel membrane stresses
- elastic shaft angle of twist
- plane-stress Mohr-circle summaries

These are reference relations, not general structural solvers. Beam formulas assume Euler-Bernoulli small-deflection behavior and constant rigidity. Euler buckling is an ideal slender-column model. Thin-wall vessel formulas require wall thickness small relative to radius and do not include discontinuities, nozzles, fatigue, plasticity, or code factors.

Existing continuum, endurance, failure, fatigue, notch, shaft, and stress APIs remain intact.
