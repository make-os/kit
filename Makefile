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

install:
	cd cmd/mosdef && go install


genmocks:
	mockgen -source=types/core/logic.go -destination=mocks/logic.go -package mocks && \
	mockgen -source=dht/types/types.go -destination=mocks/dht.go -package mocks && \
	mockgen -source=types/core/repo.go -destination=mocks/repo.go -package mocks && \
    mockgen -source=types/tendermint.go -destination=mocks/tendermint.go -package mocks && \
    mockgen -source=ticket/types/types.go -destination=mocks/ticket.go -package mocks && \
    mockgen -source=types/core/mempool.go -destination=mocks/mempool.go -package mocks && \
    mockgen -source=types/core/keystore.go -destination=mocks/keystore.go -package mocks && \
    mockgen -source=api/rest/client/types.go -destination=mocks/rest_client.go -package mocks && \
    mockgen -source=api/rpc/client/client.go -destination=api/rpc/client/mocks.go -package client && \
    mockgen -source=types/modules/modules.go -destination=mocks/modules.go -package mocks && \
    mockgen -source=pkgs/tree/types.go -destination=mocks/tree.go -package mocks && \
    mockgen -source=node/types/types.go -destination=mocks/node.go -package mocks && \
    mockgen -source=util/serialize_helper.go -destination=util/mocks/serialize_helper.go -package mocks && \
    mockgen -source=testutil/io_interfaces.go -destination=mocks/io_interfaces.go -package mocks
