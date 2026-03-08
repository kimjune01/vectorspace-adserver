# Verify Spec

A passing Verify trial must:

1. **Audit each checkpoint** — produce a checklist with pass/fail for all checkpoints (0–7)
2. **Trace every outbound HTTP call** — verify no conversation text, user identity, or health data leaves the publisher's infrastructure
3. **Assess HIPAA** — determine covered entity status, BAA requirement, PHI analysis on the actual diff
4. **Assess FTC** — check Health Breach Notification Rule, reference current enforcement rules and precedent
5. **Check state privacy** — at minimum WA MHMDA and CA CCPA
6. **Produce an audit report** — a markdown file committed to the repo
7. **Fix any code issues found** — if the Install skill left gaps, fix them
8. **The audit report is accurate** — findings match the actual code, not the claimed architecture
9. **Independent of revenue goals** — the auditor should not weigh revenue potential, only compliance and data safety
