# Sonnet written-response marking

T-0034 adds an AI-first, fail-closed marking lifecycle for submitted written attempts. A learner
can request marking only for their own `pending_review` short-answer, structured-response, or
essay attempt. Core creates one durable request per attempt, pins the question revision and
canonical rubric version, and never changes the submitted attempt itself.

Core sends the AI adapter only opaque IDs and criterion IDs/maxima. It never sends a learner
answer, rubric wording, answer key, source material, provenance, or credentials. The adapter has
no live provider in this build. Without a configured HTTPS service URL and service token, a
request remains `pending`; no mark is invented.

The only terminal states are `accepted` and `withheld`. Accepted criterion marks must exactly
match the pinned criterion set and maxima, and are immutable with model/version/cost/confidence
trace. Invalid, unavailable, or low-quality output retries up to three times then withholds.
Learner routes return only the owned lifecycle and, after acceptance, score and criterion feedback.

No human-review queue or manual marking workflow is included.
