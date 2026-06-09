# Product Glossary

ShipSystem is a training, demonstration, replay, and audit simulator only. These terms must not be used to imply real weapon-control, fire-control, electronic-warfare, radar-control, or tactical engagement behavior.

| Term | Meaning |
| --- | --- |
| Run | One simulator execution created from a scenario. A run can be started, paused, stopped, reviewed, annotated, exported, and archived as a training record. |
| Scenario | Training-only JSON input that defines the simulated ownship position, simulated sensors, zones, seeded contacts, allowed abstract actions, assessment profile, and optional record-completeness rules. |
| Snapshot | Persisted replay frame for a run. A snapshot stores simulated tracks and contacts at a sampled time so instructors can review state later. |
| Training action | Abstract learner or instructor action recorded for review, currently `maneuver`, `decoy`, or `training_response`. It is an audit event only and does not command equipment. |
| Assessment | Record-completeness review in exported reports. It checks evidence such as action count, replay coverage, metadata, notes, and annotations; it is not tactical scoring. |
| Replay anchor | URL query pair `run=<run_id>&at=<RFC3339 time>` that opens the console at the nearest persisted replay frame for that run. |
| Bookmark | Browser-local saved replay anchor used during review. Bookmarks are convenience UI state, not persisted audit evidence. |
| Archive export | Local read-only bundle created by `scripts/export-run-archive.ps1` containing report exports and run evidence before retention pruning. |
