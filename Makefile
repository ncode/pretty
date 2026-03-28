BINARY    := pretty
COMPOSE   := docker compose -f docker-compose.sshd.yml
SETUP     := ./scripts/ssh-testbed-setup.sh
CONFIG    := .pretty-test/pretty.yaml
GO_FILES  := $(shell find . -name '*.go' -not -path './.worktrees/*')

.PHONY: all build test test-race coverage vet \
        testbed testbed-setup testbed-up testbed-scan testbed-down \
        run demo clean

all: build

# --- Build ---

build: $(BINARY)

$(BINARY): $(GO_FILES) go.mod go.sum
	go build -o $(BINARY) .

# --- Test / Lint ---

test:
	go test -v ./...

test-race:
	go test -race ./...

coverage:
	go test -coverpkg=./... ./... -race -coverprofile=coverage.out -covermode=atomic

vet:
	@bad=$$(gofmt -l . | grep -v '^\.worktrees/'); test -z "$$bad" || { echo "$$bad"; echo "gofmt needed"; exit 1; }
	go vet ./...

# --- Testbed ---

testbed-setup: .pretty-test/sshd.env

.pretty-test/sshd.env:
	PRETTY_AUTHORIZED_KEY="$$(ssh-add -L)" $(SETUP)

testbed-up: testbed-setup
	$(COMPOSE) up -d --build
	@echo "Waiting for SSHD containers..."
	@for port in 2221 2222 2223; do \
		for i in 1 2 3 4 5 6 7 8 9 10; do \
			ssh-keyscan -p $$port localhost >/dev/null 2>&1 && break; \
			sleep 1; \
		done; \
	done

testbed-scan: testbed-up
	$(SETUP)

testbed: testbed-scan build

# --- Run ---

run: $(BINARY)
	./$(BINARY) --config $(CONFIG) -G testbed

demo: testbed run

# --- Cleanup ---

testbed-down:
	-$(COMPOSE) down

clean: testbed-down
	rm -rf .pretty-test
	rm -f $(BINARY) coverage.out
