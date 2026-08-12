# Teacher classes and assignments

Teachers create only owner-scoped classes. An invitation token is generated once, returned only to
the teacher at creation time, and stored only as a SHA-256 hash with an expiry and bounded use
count. A learner becomes a member only by accepting that token; they can revoke membership at any
time. Revocation removes future roster and assignment delivery access without rewriting prior
immutable records.

Assignments pin a verified, learner-eligible question selection and module at creation. Their
settings and item snapshots are immutable. `automated` is the default marking mode. The only
alternative is the explicit `manual_teacher` assignment setting; it does not create a global human
review queue or silently replace automated marking.

Teacher surfaces receive aggregate progress counts and consent status only. Learner surfaces
receive their own assignment delivery snapshots only. Neither surface contains raw answers,
rubrics, answer keys, source/provenance, feedback, model trace, or cost fields.
