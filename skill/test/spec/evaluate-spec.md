# Evaluate Spec

A passing Evaluate trial must:

1. **Ask discovery questions** — elicit the operator's revenue goals, compliance concerns, and UX priorities before analyzing code
2. **Identify the chatbot entry point** — name the file/function where user messages are handled
3. **Identify the LLM** — which provider/model is used
4. **Map the architecture** — list repos/projects involved, tech stack for each
5. **Estimate traffic** — produce a number (or explain why it can't be determined from code alone)
6. **Discuss revenue honestly** — acknowledge what's known (traffic, target rate) and what's unknown (tap-through, fill rate, CPM), frame revenue as tuneable not fixed
7. **Give a go/no-go recommendation** — with honest reasoning, acting in the publisher's interest
8. **Not modify any files** — the Evaluate skill is read-only

Note: In automated trials (no human operator), the agent should still attempt the discovery questions. The trial passes if it asks before proceeding, even though no human answers.
