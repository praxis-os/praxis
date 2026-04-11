# Review: Phase 1 — API Scope and Positioning

## Overall Assessment

Phase 1 delivers a coherent, internally consistent charter that adopts a
working position on all fourteen required decisions and resolves every one
of the seed's four open questions. The artifacts are cleanly scoped, cross-referenced, and do not
smuggle implementation detail forward into territory reserved for Phases
2–6. Two blockers surfaced in the reviewer subagent pass (an undated D10
conditional and an unreconciled error-taxonomy count change from D07) were
addressed with targeted addenda rather than decision reopens, and the
decoupling contract grep returns zero matches. The phase is ready to close.

## Critical Issues

None.

The reviewer subagent raised two blockers in its first pass; both were
resolved in place and verified clean in a follow-up verification pass. The
addenda are visible at `01-decisions-log.md` D07 (error-taxonomy amendment,
Phase 2 handoff note on state count) and D10 (Phase 3 tripwire with
`MODULE_PATH_TBD` placeholder fallback). No residual critical issue
remains.

## Important Weaknesses

1. **D10 is decided conditional, not unconditional.** The module path
   `github.com/praxis-os/praxis` is the canonical path but depends on two
   preconditions (GitHub org acquisition, brand review vs `usepraxis.app`
   in the adjacent runtime-governance space) that cannot be resolved inside
   a design phase. The tripwire in D10 prevents Phase 3 from embedding the
   name into godoc, but it does not remove the external dependency. This
   is the phase's single largest exposure to late rework.
   **Strengthen by:** naming an owner and a target resolution date in the
   next planning cycle, outside the Phase 1 artifact.

2. **Seed state-count discrepancy is flagged but not resolved.** Seed §1
   and §8 describe an "11-state machine"; seed §4.2 enumerates 13 numbered
   states (four terminal). D07 now adds `ApprovalRequired`, which the
   Phase 1 handoff note correctly records as a Phase 2 reconciliation
   item. The decision to defer is correct — state-count enumeration is
   Phase 2 scope — but the deferral leaves the seed internally inconsistent
   until Phase 2 amends it.
   **Strengthen by:** Phase 2's plan-phase step must explicitly claim the
   state-count reconciliation as an exit criterion.

3. **`budget.PriceProvider` and `identity.Signer` freeze promotion is
   softly guaranteed, not scheduled.** Both interfaces sit at
   `stable-v0.x-candidate`. The promotion language in
   `04-v1-freeze-surface.md` says "expected to promote... barring Phase 3
   discovering a method-signature issue" for `PriceProvider` and "after
   Phase 5 sign-off" for `Signer`. These are soft commitments. The release
   roadmap in the seed (§8 v0.5.0) requires both frozen before v0.5.0 tag;
   Phase 6 (Release) will need a hard gate here.
   **Strengthen by:** carry forward to Phase 3 and Phase 5 as explicit
   exit criteria, not soft expectations.

4. **D12's smoke-test promise has an unchecked boundary condition.**
   D12 guarantees zero-wiring construction with only `llm.Provider`
   supplied. The question of what the smoke path does when the caller
   supplies a streaming-capable provider but does not consume the stream
   is unaddressed — it is correctly Phase 2's concern, but the Phase 1
   artifact does not flag it for Phase 2 pickup.
   **Strengthen by:** Phase 2's plan should include a "zero-wiring
   streaming path behavior" item.

5. **The "bus factor" risk from seed §14.1 is not discussed anywhere in
   the Phase 1 artifacts.** The seed identifies a small early-maintainer
   set as a material risk. Phase 1 focuses on charter and design, so the
   omission is defensible, but R1–R8 in `00-plan.md` inherit the seed
   risks implicitly without ever naming bus-factor. This is a gap a
   reviewer of Phase 6 will notice.

## Open Questions

1. **What is the v0.1.0 content if D10 is still unresolved at Phase 3
   close?** The tripwire says Phase 3 uses `MODULE_PATH_TBD`, but the
   v0.1.0 release cuts a real module. The implicit answer is "v0.1.0
   is gated on D10 preconditions," but this is not written anywhere.

2. **Is the returned-to-caller approval model in D07 composable with
   long-running out-of-process workflows?** The decision is sound for
   short-lived approvals; for multi-hour human approval flows the caller
   must implement persistence. Does praxis document a reference pattern
   (even just an example) or leave it entirely to callers? This is a
   Phase 3 (examples) or Phase 6 (documentation) concern.

3. **Does `ApprovalRequiredError` carry enough structured data for the
   caller to implement resume?** D07 declares the error is
   `errors.As`-compatible but does not specify what fields the concrete
   type exposes (approval reason, resume token, hook name, phase). This
   is explicitly Phase 3's call, but the uncertainty is worth flagging.

4. **Does the smoke-test promise (D12) imply a behavior for
   `NullPriceProvider` returning zero cost?** If yes, the budget cost
   dimension is effectively disabled in the smoke path, which is fine
   for a 30-line example but may surprise users who do not notice.
   Phase 4 should document this.

5. **Is the Azure OpenAI compatibility matrix (D14) a Phase 3 or Phase 6
   deliverable?** D14 commits to "best-effort with a documented matrix,"
   but the document ownership is unassigned. Phase 3 (adapter design)
   is the natural home; Phase 6 (release) can also claim it.

## Decoupling Contract Check

**PASS.** A case-insensitive, word-bounded grep over
`docs/phase-1-api-scope/` against the banned-identifier set from seed §6.1
(consumer brand names, the alternative event-namespace vocabulary,
hardcoded caller-identity attribute names, milestone codes from other
repositories, consumer-branded file paths) returns zero matches.

A pre-review pass caught a false-positive substring collision where a
non-goal heading numbered with the prefix "NG" trailed by a digit matched
one of the banned milestone-code substrings. That was resolved before
reviewer-subagent verification by renaming non-goal headings to
`Non-goal N`. A follow-up grep after the D07 and D10 addenda were applied
also returned zero matches. The full banned-identifier pattern and its
literal tokens are documented in seed §6.1 — this review intentionally
does not embed the pattern inline to avoid a self-referential match.

## Recommendations

- **Close the phase.** The decisions are adopted, the contract is clean,
  the blockers are resolved. Decisions remain amendable in later phases
  per the amendment protocol in `01-decisions-log.md`.
- **Carry forward to Phase 2** as explicit plan-phase exit criteria: (a)
  state-count reconciliation (11 vs 13+1), (b) terminal-state
  representation for `ApprovalRequired` (new terminal vs typed error
  with existing terminals), (c) zero-wiring streaming path behavior
  (D12 boundary condition), (d) budget-clock handling in the edge case
  where D07's returned-to-caller model interacts with the wall-clock
  dimension.
- **Carry forward to Phase 3** as explicit plan-phase exit criteria: (a)
  `budget.PriceProvider` freeze promotion (gated on method signature
  only), (b) `ApprovalRequiredError` concrete field set, (c) Azure
  OpenAI compatibility matrix ownership, (d) module path resolution
  (D10 preconditions) before any godoc is written.
- **Carry forward to Phase 5** as explicit plan-phase exit criterion:
  `identity.Signer` freeze promotion after JWT claim set and key
  lifecycle are finalized.
- **Carry forward to Phase 6**: record bus-factor risk explicitly,
  confirm D10 preconditions have been resolved or the rename executed,
  and include the D14 best-effort Azure compatibility matrix in release
  notes.
- **Do not reopen** any of D01–D14 in later phases without an explicit
  amendment decision in the owning phase's decision log.

## Verdict: READY

Phase 1 adopts a working position on every decision it set out to decide,
resolves every seed open question, passes the decoupling contract hard
gate, and its residual weaknesses are cleanly deferrable to the phases
that own them. Any adopted position may be amended in a later phase if
justified, per the protocol recorded in `01-decisions-log.md`.
