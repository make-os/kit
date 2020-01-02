GOVERSION := $(shell go version | cut -d" " -f3)

# Run tests
test:
	go test ./...
	
# Run tests (with ginkgo)
ginkgo:
	ginkgo ./...

# Clean and format source code	
clean: 
	go vet ./... && gofmt -s -w .
	
# Ensure dep depencencies are in order
dep-ensure:
	dep ensure -v

# Install source code and binary dependencies
deps: dep-ensure
	go get github.com/gobuffalo/packr/packr

# Create a release 
release:
	env GOVERSION=$(GOVERSION) goreleaser --snapshot --rm-dist
	
# Create a tagged release 
release-tagged:
	env GOVERSION=$(GOVERSION) goreleaser release --skip-publish --rm-dist

# Create a release 
release-linux:
	cd $(GOPATH)/src/github.com/ellcrys/elld && \
	 git checkout ${b} && \
	 dep ensure -v && \
	 env GOVERSION=$(GOVERSION) goreleaser release --snapshot --rm-dist -f ".goreleaser.linux.yml"

genmocks:
	cd types && \
	mockgen -source=logic.go -destination=mocks/logic.go -package mocks && \
	mockgen -source=dht.go -destination=mocks/dht.go -package mocks && \
	mockgen -source=repo.go -destination=mocks/repo.go -package mocks && \
	mockgen -source=tm.go -destination=mocks/tm.go -package mocks && \
	mockgen -source=jsonrpc.go -destination=mocks/jsonrpc.go -package mocks && \
	mockgen -source=mempool.go -destination=mocks/mempool.go -package mocks && \
	mockgen -source=ticket.go -destination=mocks/ticket.go -package mocks && \
	mockgen -source=tree.go -destination=mocks/tree.go -package mocks