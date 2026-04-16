# Changelog

## [0.11.0](https://github.com/praxis-os/praxis/compare/v0.9.1...v0.11.0) (2026-04-16)


### Added

* add Gemini, OpenRouter, Groq, and Ollama LLM providers ([#37](https://github.com/praxis-os/praxis/issues/37)) ([7c0875f](https://github.com/praxis-os/praxis/commit/7c0875fc5046d67edfee833ae48608bce2e0d331))

## [0.9.1](https://github.com/praxis-os/praxis/compare/v0.9.0...v0.9.1) (2026-04-16)


### Testing

* coverage fills, CI multi-module expansion, submodule benchmarks ([#34](https://github.com/praxis-os/praxis/issues/34)) ([d3e592d](https://github.com/praxis-os/praxis/commit/d3e592d3d568245716cc2bbab0f1735d7cd87e9e))

## [0.9.0](https://github.com/praxis-os/praxis/compare/v0.7.0...v0.9.0) (2026-04-15)


### Added

* **skills:** implement praxis/skills sub-module (v0.9.0) ([#32](https://github.com/praxis-os/praxis/issues/32)) ([f2f606b](https://github.com/praxis-os/praxis/commit/f2f606b2f2f18d819396facd3b7f07ca26b8829b))

## [0.7.0](https://github.com/praxis-os/praxis/compare/v0.5.0...v0.7.0) (2026-04-15)


### Added

* **mcp:** S29 — scaffold praxis/mcp sub-module and extend release-please to two-package form ([#24](https://github.com/praxis-os/praxis/issues/24)) ([10ed8f6](https://github.com/praxis-os/praxis/commit/10ed8f6257fa68f43da1aa266163a675135cb2a6))
* **mcp:** S30 — define minimal praxis/mcp public API surface (Server, Transport, Invoker, New, Options) ([#25](https://github.com/praxis-os/praxis/issues/25)) ([59c8f1d](https://github.com/praxis-os/praxis/commit/59c8f1d2c4124d74b0d083d405a8915d1ee7693c))
* **mcp:** S31 PR-A — integrate modelcontextprotocol/go-sdk v1.5.0 (T31.1 + T31.6 + D107 audit) ([#26](https://github.com/praxis-os/praxis/issues/26)) ([db72169](https://github.com/praxis-os/praxis/commit/db721699c243952bfee699a18eb1c821c145d23c))
* **mcp:** S31 PR-B — stdio + Streamable HTTP transports with credential lifecycle (T31.2–T31.5) ([#27](https://github.com/praxis-os/praxis/issues/27)) ([2d1787e](https://github.com/praxis-os/praxis/commit/2d1787efb393576f4764c225965ad3111f0d2447))
* **mcp:** S32 — tool adapter, namespacing, dispatch, Definitions accessor ([#28](https://github.com/praxis-os/praxis/issues/28)) ([a86aa93](https://github.com/praxis-os/praxis/commit/a86aa93bef1bd7089b16b13c3b139a22deddd1ac))
* **mcp:** S33 — content flattening + error translation ([#29](https://github.com/praxis-os/praxis/issues/29)) ([ae84aed](https://github.com/praxis-os/praxis/commit/ae84aed5296f62964c29443b50111513d936e6aa))
* **mcp:** S34 — MCP observability (MCPMetricsRecorder + bounded-cardinality metrics) ([#30](https://github.com/praxis-os/praxis/issues/30)) ([d6903ec](https://github.com/praxis-os/praxis/commit/d6903ecf002ff4f016d811ffc77d0d9c0e2654f0))
* **mcp:** S35 — trust boundary, tests, examples & docs (v0.7.0 final) ([#31](https://github.com/praxis-os/praxis/issues/31)) ([40509f5](https://github.com/praxis-os/praxis/commit/40509f58bdaadcf46746e2fb87e767e026600c85))


### Documentation

* **planning:** approve Phase 7 & 8, reorder roadmap to 5→7→8→6, fix module path ([#23](https://github.com/praxis-os/praxis/issues/23)) ([6761b31](https://github.com/praxis-os/praxis/commit/6761b31955073552e9812074d190eb1692141afe))


### Performance

* performance remediation — reduce allocations, parallel dispatch, transport tuning ([#21](https://github.com/praxis-os/praxis/issues/21)) ([3d0f2cc](https://github.com/praxis-os/praxis/commit/3d0f2cc8b47fa0cb056915bbf2151258e850f6df))

## [0.5.0](https://github.com/praxis-os/praxis/compare/v0.3.0...v0.5.0) (2026-04-09)


### Added

* v0.5.0 — feature complete ([#19](https://github.com/praxis-os/praxis/issues/19)) ([3adaf43](https://github.com/praxis-os/praxis/commit/3adaf433d1ea48ef2ac418f5a20bb346f892fd1b))

## [0.3.0](https://github.com/praxis-os/praxis/compare/v0.1.0...v0.3.0) (2026-04-08)


### ⚠ BREAKING CHANGES

* v0.3.0 Wave 3-6 — cancel, hooks, budget, OTel, OpenAI ([#14](https://github.com/praxis-os/praxis/issues/14))
* v0.3.0 Wave 1-2 — Phase 3 interfaces + streaming ([#11](https://github.com/praxis-os/praxis/issues/11))

### Added

* v0.3.0 Wave 1-2 — Phase 3 interfaces + streaming ([#11](https://github.com/praxis-os/praxis/issues/11)) ([125a5f0](https://github.com/praxis-os/praxis/commit/125a5f0825ce4dd85c429153f56e0cf8e11f17b3))
* v0.3.0 Wave 3-6 — cancel, hooks, budget, OTel, OpenAI ([#14](https://github.com/praxis-os/praxis/issues/14)) ([1facf48](https://github.com/praxis-os/praxis/commit/1facf48f3a6dcb28eab16c4b76a21618d3cbf35b))


### Fixed

* **release:** enforce odd pre-v1 milestones ([#17](https://github.com/praxis-os/praxis/issues/17)) ([4ade85c](https://github.com/praxis-os/praxis/commit/4ade85c461741a6ca22cd8e48461343f9b4450f0))
* **release:** publish release pr statuses ([cd87046](https://github.com/praxis-os/praxis/commit/cd870461465b86e67fffcd312aeadad01c02c8b5))
* **release:** trigger ci for release prs ([1e17d62](https://github.com/praxis-os/praxis/commit/1e17d62ac723b67ba470415b680d2bd6d772c409))
* **release:** update open release prs ([652e436](https://github.com/praxis-os/praxis/commit/652e4369e92b5f03ea89f18392193da400a1813b))
* **release:** use cli for release-as ([3b71822](https://github.com/praxis-os/praxis/commit/3b71822f64c53f56f9700c9e6a5a04050dcca772))
