# SPDX-License-Identifier: Apache-2.0

.PHONY: test lint bench cover check banned-grep spdx-check fmt vet

# Module paths that make targets operate on. Resolved through go.work so
# core (.), praxis/mcp, and praxis/skills are exercised by every CI target.
MODULE_PATHS := ./... ./mcp/... ./skills/...

# Run all tests with race detection
test:
	go test -race -count=1 $(MODULE_PATHS)

# Run golangci-lint
lint:
	golangci-lint run $(MODULE_PATHS)

# Run go vet
vet:
	go vet $(MODULE_PATHS)

# Run gofmt check (fails if any files need formatting)
fmt:
	@UNFORMATTED=$$(gofmt -l .); \
	if [ -n "$$UNFORMATTED" ]; then \
	  echo "Files need gofmt:"; echo "$$UNFORMATTED"; exit 1; \
	fi
	@echo "gofmt check: PASS"

# Run benchmarks
bench:
	go test -bench=. -benchmem -count=5 $(MODULE_PATHS)

# Generate coverage report (excludes examples/, matches CI threshold)
cover:
	go test -race -coverprofile=coverage.out $$(go list $(MODULE_PATHS) | grep -v /examples/)
	go tool cover -func=coverage.out

# Check for banned identifiers (decoupling contract enforcement)
# Scope: .go and .md files. Excludes phase docs, seed context, reviews,
# tooling config (.claude/, CLAUDE.md), and the roadmap-status index
# which all define or report on the rules themselves.
banned-grep:
	@echo "Checking for banned identifiers..."
	@BANNED='custos|reef|governance.event|governance_event'; \
	RESULT=$$(grep -rniw -E "$$BANNED" --include='*.go' --include='*.md' \
	  --exclude-dir='phase-*' \
	  --exclude-dir=.claude \
	  --exclude='PRAXIS-SEED-CONTEXT.md' \
	  --exclude='REVIEW.md' \
	  --exclude='CLAUDE.md' \
	  --exclude='README.md' \
	  --exclude='roadmap-status.md' \
	  . || true); \
	if [ -n "$$RESULT" ]; then \
	  echo "BANNED IDENTIFIER FOUND:"; echo "$$RESULT"; exit 1; \
	fi
	@echo "Checking for hardcoded identity attributes..."
	@ATTRS='org\.id|agent\.id|user\.id|tenant\.id'; \
	RESULT=$$(grep -rn -E "$$ATTRS" --include='*.go' . || true); \
	if [ -n "$$RESULT" ]; then \
	  echo "HARDCODED IDENTITY ATTRIBUTE FOUND:"; echo "$$RESULT"; exit 1; \
	fi
	@echo "Banned-identifier check: PASS"

# Verify SPDX headers on all Go files
spdx-check:
	@missing=$$(find . -name '*.go' -not -path './vendor/*' \
	  | xargs grep -L 'SPDX-License-Identifier: Apache-2.0'); \
	if [ -n "$$missing" ]; then \
	  echo "Missing SPDX header in:"; echo "$$missing"; exit 1; \
	fi
	@echo "SPDX check: PASS"

# Run all checks (CI gate)
check: lint test banned-grep spdx-check
	@echo "All checks passed."
