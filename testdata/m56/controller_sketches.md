# M56 Controller Sketches — Interruption/Resume Pressure Probes

These are intentionally lightweight authored-controller sketches used to pressure-test interruption/resume semantics. They are **not** implementation artifacts.

## A. Tactical Interruption (Game-Shaped)

### A1. Engage → Investigate Noise → Resume Prior Tactic
- Baseline tactical modes: `Flank`, `Suppress`, `CloseAssault`
- Interruption: brief `InvestigateNoise`
- Desired return: resume **exact prior tactical mode**, not a fixed default

### A2. Attack Style → Reload/Dodge → Resume Attack Style
- Baseline tactical modes: `BurstFire`, `SingleShot`, `PeekFire`
- Interruption: `Reload` or `Dodge`
- Desired return: restore whichever attack style was active before interruption

### A3. Short Cover Behavior → Resume Prior Engagement
- Baseline tactical modes: `Advance`, `HoldAngle`, `PressureLeft`
- Interruption: temporary `TakeCover`
- Desired return: restore prior tactical intent quickly and predictably

## B. Industrial Temporary Override (Controller-Shaped)

### B1. Process Control → Inspection Override → Resume Prior Mode
- Baseline control modes: `HeatRamp`, `SteadyState`, `CoolDown`
- Interruption: short `Inspection`
- Desired return: resume prior process mode if preconditions still valid

### B2. Safety Hold → Return to Prior Operating Mode
- Baseline control modes: `RunAuto`, `RunManualTrim`, `IdleStandby`
- Interruption: `SafetyHold`
- Desired return: resume prior mode only when safety clear + validity checks pass

### B3. Calibration Check → Resume Current Process Stage
- Baseline stages: `DoseA`, `DoseB`, `Mix`
- Interruption: `CalibrationCheck`
- Desired return: continue interrupted stage if restart cost is high and stage context still valid

## C. False Pressure Check

### C1. Fault Recovery Should Restart Known State
- Pattern: fault detected during arbitrary mode
- Better behavior: explicit transition to `RecoverInit`, then deterministic restart path
- Anti-pattern: “resume previous” can re-enter unsafe or stale state

### C2. Operator Requested Rebaseline
- Pattern: manual request to reset process assumptions
- Better behavior: explicit `goto RebaselineStart`
- Anti-pattern: context resume preserves assumptions user explicitly invalidated

## D. Pathological Overreach Check

### D1. Deeply Nested Tactical Interruptions
- Example chain: `Flank -> Dodge -> InvestigateNoise -> Reload -> Peek`
- Pressure: temptation for unlimited `push/pop`
- Risk: invisible control stack and hard-to-audit behavior

### D2. Framework-Shaped HFSM Trees
- Pattern: architecture starts around stack machinery rather than domain control truth
- Risk: authors model hierarchy for mechanism convenience, not plant/game reality

### D3. Recursive “Push Everything” Habit
- Pattern: every interruption becomes a push, every completion a pop
- Risk: hides explicit intent, increases surprise at runtime, weakens Oct explicit style
