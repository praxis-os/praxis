# SPDX-License-Identifier: Apache-2.0

.PHONY: test lint bench cover check banned-grep spdx-check fmt vet

# Run all tests with race detection
test:
	go test -race -count=1 ./...

# Run golangci-lint
lint:
	golangci-lint run ./...

# Run go vet
vet:
	go vet ./...

# Run gofmt check (fails if any files need formatting)
fmt:
	@UNFORMATTED=$$(gofmt -l .); \
	if [ -n "$$UNFORMATTED" ]; then \
	  echo "Files need gofmt:"; echo "$$UNFORMATTED"; exit 1; \
	fi
	@echo "gofmt check: PASS"

# Run benchmarks
bench:
	go test -bench=. -benchmem -count=5 ./...

# Generate coverage report
cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Check for banned identifiers (decoupling contract enforcement)
# Scope: .go and .md files. Excludes phase docs, seed context, reviews,
# and tooling config (.claude/, CLAUDE.md) which define the rules themselves.
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
