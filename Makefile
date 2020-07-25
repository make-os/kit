GOVERSION := $(shell go version | cut -d" " -f3)

# Run tests
test:
	go test ./...
	
# Run tests (with ginkgo)
ginkgo:
	ginkgo ./...

# Install source code and binary dependencies
deps:
	go get github.com/gobuffalo/packr/packr

# Create a release 
release:
	env GOVERSION=$(GOVERSION) goreleaser --snapshot --rm-dist
	
# Create a tagged release 
release-tagged:
	env GOVERSION=$(GOVERSION) goreleaser release --rm-dist


install:
	cd cmd/lobe && go install

vet:
	go vet ./...

genmocks:
	mockgen -destination=mocks/remote_types.go -package mocks gitlab.com/makeos/lobe/remote/types LiteGit,LocalRepo,Commit
	mockgen -source=types/core/logic.go -destination=mocks/logic.go -package mocks
	mockgen -source=types/core/remote.go -destination=mocks/remote.go -package mocks
	mockgen -source=types/core/mempool.go -destination=mocks/mempool.go -package mocks
	mockgen -source=remote/push/types/objects.go -destination=mocks/pushpool.go -package mocks
	mockgen -source=remote/server/types.go -destination=mocks/servertypes.go -package mocks
	mockgen -source=remote/fetcher/objectfetcher.go -destination=mocks/object_fetcher.go -package mocks
	mockgen -source=remote/push/push_handler.go -destination=mocks/push_handler.go -package mocks
	mockgen -source=remote/types/pruner.go -destination=mocks/pruner.go -package mocks
	mockgen -source=api/remote/client/client.go -destination=mocks/rests_client.go -package mocks
	mockgen -source=api/remote/api.go -destination=mocks/rests_api.go -package mocks
	mockgen -source=remote/plumbing/post.go -destination=mocks/post.go -package mocks
	mockgen -source=api/rpc/client/client.go -destination=mocks/rpc/client.go -package mocks
	mockgen -source=rpc/server.go -destination=mocks/rpc/rpc_server.go -package mocks
	mockgen -source=dht/server/types/types.go -destination=mocks/dht_server.go -package mocks
	mockgen -source=dht/streamer/requester.go -destination=mocks/dht_requester.go -package mocks
	mockgen -source=dht/streamer/types/types.go -destination=mocks/dht_streamer.go -package mocks
	mockgen -source=ticket/types/types.go -destination=mocks/ticket.go -package mocks
	mockgen -source=keystore/types/types.go -destination=mocks/keystore.go -package mocks
	mockgen -source=modules/types/modules.go -destination=mocks/modules.go -package mocks
	mockgen -source=node/services/service.go -destination=mocks/node_service.go -package mocks
	mockgen -source=types/libp2p.go -destination=mocks/libp2p.go -package mocks
	mockgen -source=pkgs/tree/types.go -destination=mocks/tree.go -package mocks
	mockgen -source=node/types/types.go -destination=mocks/node.go -package mocks
	mockgen -source=util/serialize_helper.go -destination=util/mocks/serialize_helper.go -package mocks
	mockgen -source=testutil/io_interfaces.go -destination=mocks/io_interfaces.go -package mocks
