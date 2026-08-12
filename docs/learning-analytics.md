# Learning analytics foundation

`GET /learner/analytics` is an owner-scoped learner route. It returns only cumulative marks,
module/syllabus outcome totals, pending/withheld counts, and recent outcome labels.

## What is counted

- Deterministic submissions add one immutable scored event immediately.
- Written submissions add an unscored pending event. Only an accepted automated result adds a
  scored event; a withheld result remains visibly unscored.
- Scores are sums of completed scored outcomes only. Pending and withheld work never changes a
  marks denominator.

The database stores immutable outcome snapshots including the module label/code at the time of
submission. Aggregates are reconstructed from those events, so current editorial lifecycle or
module changes cannot rewrite learner history. Event records never contain a raw answer, question
prompt, rubric, canonical answer, source/provenance, feedback, model identity, cost, or trace.

The browser reaches this route solely through the fixed `getLearnerAnalytics` BFF operation.
There are no filters or IDs supplied by the browser, no cross-user route, and no rankings.
