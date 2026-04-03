# Repo Structure

This repository separates work by **purpose**, not by temporary convenience.

The three main homes are:

- `Libraries/`
- `Experiments/`
- `Product/`

A good rule of thumb:

- **Libraries optimize for reuse**
- **Experiments optimize for learning**
- **Product optimizes for delivery**

---

## Libraries

`Libraries/` contains **reusable reference code for stable knowledge**.

These are implementations that humans and LLMs should be able to import instead of repeatedly re-deriving from scratch. In many cases they are grounded in durable structure such as mathematics, physics, numerics, or other long-lived technical contracts.

### Libraries are for
- reusable code
- reference implementations
- stable abstractions
- importable building blocks
- things that should outlive any one experiment

### Libraries are not for
- exploratory milestone history
- messy scratchpad work
- temporary probes
- product packaging concerns

### Example
- `Libraries/Mechanics/Continuum/` for reusable Continuum reference artifacts

---

## Experiments

`Experiments/` contains **research workspaces inside the Oct repo**.

These are milestone-driven exploratory projects used to discover, test, and pressure ideas. They are allowed to be somewhat messy because their job is to help us learn, not to look like polished packages.

### Experiments are for
- research
- milestone-based probing
- exploratory code and reports
- scratchpad work
- boundary testing
- discovery before synthesis

### Experiment format
Experiments typically use milestone folders such as:
- `M0`
- `M1`
- `M2`
- ...

These milestone folders are the working strata of the research. They may contain:
- reports
- manifests
- Oct tests
- probe code
- intermediate structures

That is acceptable.

`M<number>[letter]` folders are formal milestones with a stricter structure, while `Mx<number>[letter]` folders are exploratory/scratch work with looser internal structure.

### Experiments are not for
- finalized reusable library organization
- public package structure
- long-term product-facing polish

### Note on Archive
We may eventually add an `Archive/` area for fully completed experiments, but there is no need for that yet.

### Example
- `Experiments/ContinuumComputabilityBoundary/`

---

## Product

`Product/` contains **usable deliverables lowered from Experiments into coherent package form**.

This is where research becomes something that can actually be used, shipped, sold, or built upon directly.

The first product format is expected to be an Oct package, but `Product/` is **not restricted to Oct forever**. Oct is the reference format for now; future product outputs may exist in other forms when justified.

### Product is for
- synthesized deliverables
- package-ready architecture
- operational defaults
- coherent end-to-end implementations
- things intended for actual use

### Product is not for
- raw milestone history
- unresolved research scratchpads
- language-feature work that belongs under `Language/`

### Example
- `Product/Mechanics/Continuum/`

---

## Language

`Language/` is for **Oct language features, syntax, semantics, and core language behavior**.

If a milestone is about the language itself, it belongs here, even if it was discovered during another experiment.

### Language is for
- syntax
- semantics
- type system behavior
- control flow and expression features
- core builtins and language-level contracts

### Example
- `Language/Expressions/UtilityWhen/`

### Important rule
A milestone should only live under `Language/` if it is truly a **language milestone**.

Do not put mechanics experiment history under `Language/` just because the implementation happens to be written in Oct.

---

## How work should move

A healthy progression usually looks like this:

### 1. Experiment
An idea starts in `Experiments/` where it can be tested honestly.

### 2. Library and/or Product
Once the idea stabilizes:
- reusable stable pieces move into `Libraries/`
- synthesized usable deliverables move into `Product/`

### 3. Language, if truly needed
If the work reveals an actual Oct language feature or semantics change, that part belongs in `Language/`.

---

## Continuum-specific interpretation

For the Continuum line of work:

- **Probe-style milestone history** belongs in  
  `Experiments/ContinuumComputabilityBoundary/`

- **Reusable stable reference artifacts** belong in  
  `Libraries/Mechanics/Continuum/`

- **Synthesized usable package form** belongs in  
  `Product/Mechanics/Continuum/`

- **Language features discovered during the work** belong in  
  `Language/...`

Example:
- the `when utility` milestone is a **language milestone**, not a Continuum mechanics milestone

---

## Placement rules

When deciding where something belongs, ask:

### Put it in `Libraries/` if:
- it is reusable
- it is stable
- it should be imported, not rediscovered

### Put it in `Experiments/` if:
- it is exploratory
- it is milestone-driven
- it exists to answer research questions

### Put it in `Product/` if:
- it is a synthesized deliverable
- it is intended for practical use
- it has crossed out of scratchpad mode

### Put it in `Language/` if:
- it changes or defines Oct itself

---

## Anti-patterns

Avoid these mistakes:

### 1. Experiment clutter in Libraries
Do not move raw milestone archaeology into `Libraries/` just because it feels important.

### 2. Mechanics research in Language
Do not put experiment history under `Language/` unless the milestone is actually about Oct.

### 3. Productization too early
Do not put unfinished exploratory work into `Product/`.

### 4. Re-deriving stable knowledge
If something is already a stable reusable reference, it belongs in `Libraries/`, not reimplemented ad hoc everywhere.

---

## Final principle

The repo should reflect **semantic ownership**.

Not:
- where something was first hacked together
- where it was temporarily convenient
- where Codex happened to put it

But:
- what the thing **is for**
- what kind of work it represents
- and how it is meant to be used
