# Changelog

## [0.1.1](https://github.com/praxis-os/praxis/compare/v0.1.0...v0.1.1) (2026-04-08)


### Added

* add foundational design document for praxis library ([0d4bf4a](https://github.com/praxis-os/praxis/commit/0d4bf4af1c5c08a572269851fbeee31aca2154d8))
* add null/noop defaults for all v1.0 interfaces (T6.1–T6.7) [PRAX-61 PRAX-62 PRAX-63 PRAX-64 PRAX-65 PRAX-66 PRAX-67] ([49abc27](https://github.com/praxis-os/praxis/commit/49abc27b7727e7377d5ae436ea40bedd44de5927))
* **anthropic:** implement Anthropic Messages API provider [PRAX-58] ([3557eef](https://github.com/praxis-os/praxis/commit/3557eef92ac0df44b96b4c08f1669930cd83e9d3))
* **build:** add Makefile and golangci-lint config [PRAX-38] ([f17bde5](https://github.com/praxis-os/praxis/commit/f17bde54560524461fb62b0d88322661af3f8e0d))
* **build:** create go.mod with confirmed module path [PRAX-37] ([486fed6](https://github.com/praxis-os/praxis/commit/486fed6f615e69e96e143e54d7d42a0e67181825))
* **docs:** add Phase 6 governance and release milestones documentation ([07d5c8e](https://github.com/praxis-os/praxis/commit/07d5c8e00eec3fe88fd34bb12d9fd8b2f9805b5e))
* **docs:** add SKILL and jira-map documentation for task management and execution ([b9235c1](https://github.com/praxis-os/praxis/commit/b9235c1d77e08afa92d50237bee26193a0ad0b4c))
* **docs:** enhance SKILL.md with Confluence configuration and update procedure ([d543440](https://github.com/praxis-os/praxis/commit/d5434404a09f94b95f921ac58386474203e19027))
* **errors:** add 8 concrete error types [PRAX-53] ([32abb56](https://github.com/praxis-os/praxis/commit/32abb56ac56897d052876ea90854f8f82a5887a5))
* **errors:** add 8 concrete error types [PRAX-53] ([92d97ea](https://github.com/praxis-os/praxis/commit/92d97ea94dcfcf13f68040fbf8c909202d3f2ace))
* **errors:** add DefaultClassifier and RetryPolicy [PRAX-54] ([285793c](https://github.com/praxis-os/praxis/commit/285793c104fb0f65114467d401abed2fb75c9e8f))
* **errors:** add DefaultClassifier and RetryPolicy [PRAX-54] ([dbee790](https://github.com/praxis-os/praxis/commit/dbee79059d2c4d97773aa936fbb7cfa24ec3c838))
* **errors:** add TypedError interface and ErrorKind [PRAX-52] ([a828fd9](https://github.com/praxis-os/praxis/commit/a828fd9fefd6df74935d357241b3c43c4103406d))
* **errors:** add TypedError interface, ErrorKind, and Classifier [PRAX-52] ([8f5d29e](https://github.com/praxis-os/praxis/commit/8f5d29ece6f9981b15655ddb233e8581a995b4cb))
* **invocation:** add InvocationRequest and InvocationResult types [PRAX-47] ([6b249f8](https://github.com/praxis-os/praxis/commit/6b249f8a6b69128493f583b96407c7c47ad545b1))
* **llm:** add Provider interface and message/tool types [PRAX-56, PRAX-57] ([dba7bcf](https://github.com/praxis-os/praxis/commit/dba7bcf08b29839d8ce05c48bbbf7d60a7e26c94))
* **llm:** add Provider interface and message/tool types [PRAX-56, PRAX-57] ([6f25f33](https://github.com/praxis-os/praxis/commit/6f25f3369185474e649806c94a969c55398d862a))
* **mock:** add configurable mock LLM provider for testing [PRAX-59] ([72e8568](https://github.com/praxis-os/praxis/commit/72e85685eaba33113d24eb4a6e095f30818d351e))
* **orchestrator:** add New constructor with functional options [PRAX-48] ([db5a859](https://github.com/praxis-os/praxis/commit/db5a8598dd6a7d6fe0729992d3c2e4192b572d51))
* **orchestrator:** add With* option functions for all v1.0 interfaces [PRAX-50] ([ea0a866](https://github.com/praxis-os/praxis/commit/ea0a866f78e4922a4ff188bdc914fcaa333ac023))
* **orchestrator:** implement invocation loop driver [PRAX-49] ([00cdc62](https://github.com/praxis-os/praxis/commit/00cdc623bcce373355c8b6f3b458758a75677eee))
* **retry:** add internal/retry with exponential backoff and jitter [PRAX-55] ([830e383](https://github.com/praxis-os/praxis/commit/830e383f5eff6f7a5b9637e179f52bcfef050ff2))
* **state:** add Machine type with transition enforcement [PRAX-44] ([f369dec](https://github.com/praxis-os/praxis/commit/f369decf1da94a818cd6acd7a9a4fd343a929109))
* **state:** define State type with 14 constants and transition table [PRAX-42] ([bec3b77](https://github.com/praxis-os/praxis/commit/bec3b771fbc911677a57dfd239768300e713d28c))
* wave 5 — docs, CI, quality gate, and release config [PRAX-39 PRAX-41 PRAX-60 PRAX-68 PRAX-69 PRAX-70 PRAX-71 PRAX-72 PRAX-73 PRAX-74 PRAX-75 PRAX-76] ([c73044d](https://github.com/praxis-os/praxis/commit/c73044d1d4dedd75fe662ef140c4c25ac1066ebf))


### Fixed

* **module:** update module path to correct repository location ([6ca617f](https://github.com/praxis-os/praxis/commit/6ca617f27515a346c61eac51eb717a1782f2596c))


### Documentation

* add Jira decomposition with Epic/Story/Task structure for v0.1–v1.0 ([52c3c42](https://github.com/praxis-os/praxis/commit/52c3c429ac70d044a778d8b7fede1147ec32dc8b))
* add Phase 5 validation document for Security and Trust Boundaries ([8a7cca4](https://github.com/praxis-os/praxis/commit/8a7cca42729a261a5e2e74de9c211c9c8b1d37fd))
* update jira-map with task completion status ([a02a16a](https://github.com/praxis-os/praxis/commit/a02a16a354ab74820d7a175ea1cfe544f163d99b))


### Testing

* **orchestrator:** add comprehensive e2e tests for sync Invoke [PRAX-51] ([6662108](https://github.com/praxis-os/praxis/commit/666210804f5f3c38499a816df8718a4157e6dd16))
* **state-machine:** add 21 invariant tests from D28 (INV-01 through INV-21) [PRAX-46] ([d43fc39](https://github.com/praxis-os/praxis/commit/d43fc39a085ffad826deb5efe9434e93c5bb012c))
* **state:** add property-based state machine tests at 10k iterations [PRAX-45] ([3e11e5b](https://github.com/praxis-os/praxis/commit/3e11e5b573335a633ce3962da3a795e57a5a9d97))
