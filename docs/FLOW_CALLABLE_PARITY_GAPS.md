# FLOW callable parity gaps after callable-consumers M0

CALLABLE-CONSUMERS-M0 made no FLOW implementation changes.

A bounded audit probe passed in interpreted and compiled execution for both:

1. a captured `fn(Int) -> Int` passed as a flow parameter and invoked in a state;
2. the same captured callable passed from a flow state into an early-specialized generic eager map consumer.

No concrete FLOW callable parity failure was discovered in that bounded M0
probe. This is not a claim of exhaustive FLOW parity: resumable turns,
checkpoint serialization, yielded values, board storage of function values,
and the broader callable type matrix were not expanded here because they belong
to the dedicated next milestone. The probe was diagnostic-only and was not
retained as a new FLOW contract in this milestone.
