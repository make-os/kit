GOVERSION := $(shell go version | cut -d" " -f3)

# Run tests
test:
	go test ./...

vet:
	go vet ./...

# Run tests (with ginkgo)
ginkgo:
	ginkgo ./...

# Create a release 
release:
	env GOVERSION=$(GOVERSION) goreleaser --snapshot --rm-dist

# Create a tagged release 
release-tagged:
	env GOVERSION=$(GOVERSION) goreleaser release --rm-dist

# Install from source
install:
	cd cmd/lobe && go install

getbin:
	curl -L https://storage.googleapis.com/lobe-bin/lobe_$(v)_Linux_x86_64.tar.gz | tar -xz
	mv ./lob /usr/bin/lob

# Initialize node for testnet-v1
testnet-v1-init:
	lob init -v 47shQ9ihsZBf2nYL6tAYR8q8Twb47KTNjimowxaNFRyGPL93oZL,48LZFEsZsRPda1q2kiNZKToiTaeSx63GJdq6DWq9m9C4mSvWhHD,48pFW5Yd5BLm4EVUJW8g9oG1BkNQz4wp2saLB8XmkvMRwRAB2FH,48GKXaSLgJ5ox2C1jDshFGtD6Y4Zhd1doxK6iTDp3KCSZjzdWKt -k $(vKey) -t 1595700581 --net=2000

# Build and run a docker container that runs a pre-built binary located in ./dist and connects to testnet-v1
join: testnet-v1-init
	docker build -t makeos/lobe -f docker/testnet-v1/Dockerfile --build-arg version=$(v) .
	docker start makeos || docker run --name=makeos -p 9000:9000 -p 9001:9001 -p 9002:9002 -p 9003:9003 -d makeos/lobe
	docker logs -f makeos --tail=1000

# Build and run a docker container that runs a pre-built binary located in ./dist and connects to testnet-v1
join-dist: testnet-v1-init
	docker build -t makeos/lobe -f docker/testnet-v1/Dockerfile.dist --build-arg version=$(v) .
	docker start makeos || docker run --name=makeos -p 9000:9000 -p 9001:9001 -p 9002:9002 -p 9003:9003 -d makeos/lobe
	docker logs -f makeos --tail=1000

# Build and run a docker container that builds a binary from local source and connects to testnet-v1
join-src: testnet-v1-init
	docker build -t makeos/lobe -f docker/testnet-v1/Dockerfile.source --build-arg version=$(v) .
	docker start makeos || docker run --name=makeos -p 9000:9000 -p 9001:9001 -p 9002:9002 -p 9003:9003 -d makeos/lobe
	docker logs -f makeos --tail=1000

# Build and run a docker container that builds a git a binary from the official git repository and connects to testnet-v1.
join-gitsrc: testnet-v1-init
	docker build -t makeos/lobe -f docker/testnet-v1/Dockerfile.git.source --build-arg branch=$(branch) .
	docker start makeos || docker run --name=makeos -v=$(volume) -p 9000:9000 -p 9001:9001 -p 9002:9002 -p 9003:9003 -d makeos/lobe
	docker logs -f makeos --tail=1000


genmocks:
	mockgen -destination=mocks/remote_types.go -package mocks github.com/make-os/lobe/remote/types LiteGit,LocalRepo,Commit
	mockgen -source=types/core/logic.go -destination=mocks/logic.go -package mocks
	mockgen -source=storage/types/types.go -destination=storage/mocks/types.go -package mocks
	mockgen -source=types/core/remote.go -destination=mocks/remote.go -package mocks
	mockgen -source=types/core/mempool.go -destination=mocks/mempool.go -package mocks
	mockgen -source=remote/push/types/objects.go -destination=mocks/pushpool.go -package mocks
	mockgen -source=remote/server/types.go -destination=mocks/servertypes.go -package mocks
	mockgen -source=remote/fetcher/object_fetcher.go -destination=mocks/object_fetcher.go -package mocks
	mockgen -source=remote/refsync/types/types.go -destination=mocks/refsync.go -package mocks
	mockgen -source=remote/push/push_handler.go -destination=mocks/push_handler.go -package mocks
	mockgen -source=remote/plumbing/post.go -destination=mocks/post.go -package mocks
	mockgen -source=dht/types/dht.go -destination=mocks/dht_server.go -package mocks
	mockgen -source=dht/types/provider_tracker.go -destination=mocks/provider_tracker.go -package mocks
	mockgen -source=dht/types/announcer.go -destination=mocks/announcer.go -package mocks
	mockgen -source=dht/streamer/requester.go -destination=mocks/dht_requester.go -package mocks
	mockgen -source=dht/streamer/types/types.go -destination=mocks/dht_streamer.go -package mocks
	mockgen -source=ticket/types/types.go -destination=mocks/ticket.go -package mocks
	mockgen -source=keystore/types/types.go -destination=mocks/keystore.go -package mocks
	mockgen -source=modules/types/types.go -destination=mocks/modules.go -package mocks
	mockgen -source=node/services/service.go -destination=mocks/node_service.go -package mocks
	mockgen -source=types/libp2p.go -destination=mocks/libp2p.go -package mocks
	mockgen -source=pkgs/tree/types.go -destination=mocks/tree.go -package mocks
	mockgen -source=rpc/types/types.go -destination=mocks/rpc/types.go -package mocks
	mockgen -source=util/serialize_helper.go -destination=util/mocks/serialize_helper.go -package mocks
	mockgen -source=util/wrapped_cmd.go -destination=util/mocks/wrapped_cmd.go -package mocks
	mockgen -source=testutil/io_interfaces.go -destination=mocks/io_interfaces.go -package mocks

