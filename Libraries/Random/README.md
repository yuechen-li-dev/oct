# Random

`Random` owns reproducible pseudorandom generation and small sampling-oriented helpers: seeding, uniform and Gaussian draws, distribution sampling, coin tosses, and dice.

It intentionally does not own deterministic PDF/CDF/PMF evaluation; use [`Distributions`](../Distributions/README.md) for that. Use [`Statistics`](../Statistics/README.md) to summarize samples and [`Uncertainty`](../Uncertainty/README.md) for measurement-uncertainty propagation. Start with `Random.Core.octest` for seeded reproducibility, then `Random.Distributions.octest` for sampling examples.

The package is suitable for bounded scientific experiments that record their seed. It is not a cryptographic random-number source.
