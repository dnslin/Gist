<!-- TRELLIS:START -->

# Trellis Instructions

These instructions are for AI assistants working in this project.

This project is managed by Trellis. The working knowledge you need lives under `.trellis/`:

- `.trellis/workflow.md` — development phases, when to create tasks, skill routing
- `.trellis/spec/` — package- and layer-scoped coding guidelines (read before writing code in a given layer)
- `.trellis/workspace/` — per-developer journals and session traces
- `.trellis/tasks/` — active and archived tasks (PRDs, research, jsonl context)

If a Trellis command is available on your platform (e.g. `/trellis:finish-work`, `/trellis:continue`), prefer it over manual steps. Not every platform exposes every command.

If you're using Codex or another agent-capable tool, additional project-scoped helpers may live in:

- `.agents/skills/` — reusable Trellis skills
- `.codex/agents/` — optional custom subagents

Managed by Trellis. Edits outside this block are preserved; edits inside may be overwritten by a future `trellis update`.

<!-- TRELLIS:END -->

.rules

<!-- spexcode:start -->
Commit your spec node and the code it justifies BEFORE you declare done or propose merge — the commit comes first, never as an afterthought to a declaration.

A spec body is a living current-state document: it states the node's PRESENT intent and is rewritten in place. Never accrete a "## vN" changelog heading, and never add current-state or verdict sections — version history is git's job, not the body's.

An independently-scoped feature gets its OWN spec node: if you build something separately scoped while working, create a sibling node for it rather than bundling it into your assigned node's commit (cosmetic polish riding along is the smell).

Keep the loss signal honest for what you changed — yatsu is the signal the optimizer reads, so a gap is a blind spot. Changed a node that carries a `yatsu.md`? Re-measure it: run its scenario, compare to the expected, and file the result with `spex yatsu eval <node>`. Made an obvious frontend change to a node with NO `yatsu.md`? Give it one — a scenario (description + expected) — so its loss can be measured. A frontend scenario is measured through the **actual running product** — drive a real browser, read the real DOM and capture a screenshot (or video), never reason about the code — and that real observation is filed as the reading, not left as an ad-hoc check you ran but never recorded. `spex yatsu scan --changed` shows the gaps in exactly the nodes you touched.

Don't reverse-engineer the file formats: `spex guide spec` and `spex guide yatsu` print the full spec.md and yatsu.md schema on demand. This prompt is the clue; that manual carries the detail. The CLI explains itself the same way: `spex help` is the command map (grouped by the loop each verb serves), `spex help <command>` one command's usage — when unsure of a verb, ask the tool, don't guess.

When you open a GitHub issue, link it to the spec node(s) it serves by adding a line to the issue **body**: `Spec: <node-id>` (comma-separate several). The id is the node's **leaf** name — the folder under `.spec/…/<id>/spec.md`, e.g. `sessions`, never the slash-path. An unrecognized id silently links nothing, so use a real node id (`spex board` lists them). A pull request needs no marker: opening it from your `node/<id>` branch links it for free.

## Memory hygiene — keep the shared store identity-clean

SpexCode's agent memory is keyed by the **main project**, so every agent running under this project — the main checkout AND every worktree — reads the **same** memory. That makes session- and role-specific facts toxic: one agent's note silently becomes every agent's belief. So, when deciding whether to record a memory:

- **Never record session-specific content** — this task, this worktree's transient state, a one-off decision, who you're talking to right now. Memory is ONLY for durable, cross-session project/user facts.
- **On a non-main worktree** (you are on a `node/<id>` branch, not the main checkout): do **not** record any memory for this session at all. Its work is transient and will merge or close; a durable lesson is recorded later, from main, once it has actually landed.
- **Even on main, never record a transient ROLE or IDENTITY** — "I am the supervisor", "I'm the coordinator", "I'm the agent doing X". These are per-launch facts, not durable ones. Recording one makes the next launched agent read *itself* as that role, and several agents in one folder dissolve into mutual-supervision confusion (everyone thinks they're the supervisor, everyone watches everyone).

## Reproduce before you fix — the fix's proof is a fail→pass pair

If your task is to FIX A BUG, reproduce it *first*, as a measurement — before you touch the fix. A claim that something is broken is worth nothing until the loss signal shows it broken; a claim that you fixed it is worth nothing until the same signal shows it passing. So a bug fix is bracketed by two readings of ONE scenario:

- **A — reproduce (fail).** Find the yatsu scenario whose expected the bug violates (if none fits, ADD one to the node's `yatsu.md` — a description + the expected correct behaviour), run it, and file the failing reading with evidence that SHOWS the bug: `spex yatsu eval <node> --scenario <s> --fail --note "<what's wrong>"` plus an `--image`/`--video` of the actual broken behaviour. This is not ceremony — reproducing is how you learn what actually breaks, and a fix aimed at an unreproduced bug aims at a guess.
- **B — fix, then re-measure (pass).** Make the code honor the spec, run the SAME scenario again, and file the passing reading with evidence of the corrected behaviour: `spex yatsu eval <node> --scenario <s> --pass`.

The two readings on the same scenario are the **A/B** — the error→correct transition, the fix's proof-of-work. yatsu keeps per-scenario reading history, so the pair is durable and navigable end to end.

Don't skip A because the fix looks obvious — an obvious fix with no reproduced failure leaves the loss signal blind to exactly the regression you just closed. This does not apply to building new intent (there is no prior failure to reproduce) — it is the discipline for **repair**: keep the loss signal honest across a bug's whole lifecycle, not just at the end.
<!-- spexcode:end -->
