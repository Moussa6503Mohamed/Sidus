# Assessment sessions

T-0032 makes Exam/Test runs server-owned.

- Core creates immutable learner-safe item snapshots and pins one learner attempt per item.
- Only authenticated owner can read, save, submit, or read final results.
- Exam has one open session per learner, server deadline, versioned response saves, and atomic final submission.
- Feedback remains absent from open-session projections. It is returned only through final result routes.
- Practice remains available as individual attempts; session API also supports persistent `practice` mode for future resume UI.

No source metadata, rubric keys, provenance, or private content is exposed through session payloads.
