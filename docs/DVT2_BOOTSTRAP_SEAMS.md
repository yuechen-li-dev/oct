# DVT-2 bootstrap seams

Runtime authority is typed memory records and the closed C ABI—not JSON. The ABI accepts BF16 image tokens `[1,1024,3840]`, FP32 context `[1,32,3840]`, BF16 timestep conditioning `[1,256]`, and returns FP32 image tokens `[1,1024,3840]` for exactly one denoising evaluation.

The complete frozen contracts, ownership, generations, replay identity, scaling, and artifact policy are in `internal/prometheus/DevelopmentReport/artifacts/Dvt2PreM0/dvt2_bootstrap_seams.json`. Those contracts are intentionally narrow so conditioning, scheduler, final projection, decoder, and writer can be replaced independently.
