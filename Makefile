# Fetch git latest tag
LATEST_GIT_TAG:=$(shell git describe --tags $(git rev-list --tags --max-count=1))
VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')

ldflags = -X github.com/bttcprotocol/delivery/version.Name=delivery \
		  -X github.com/bttcprotocol/delivery/version.ServerName=deliveryd \
		  -X github.com/bttcprotocol/delivery/version.ClientName=deliverycli \
		  -X github.com/bttcprotocol/delivery/version.Version=$(VERSION) \
		  -X github.com/bttcprotocol/delivery/version.Commit=$(COMMIT) \
		  -X github.com/cosmos/cosmos-sdk/version.Name=delivery \
		  -X github.com/cosmos/cosmos-sdk/version.ServerName=deliveryd \
		  -X github.com/cosmos/cosmos-sdk/version.ClientName=deliverycli \
		  -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
		  -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT)

BUILD_FLAGS := -ldflags '$(ldflags)'

clean:
	rm -rf build

tests:
	# go test  -v ./...

	go test -v ./app/ ./auth/ ./clerk/ ./sidechannel/ ./bank/ ./chainmanager/ ./topup/ ./checkpoint/ ./staking/ -cover -coverprofile=cover.out


build: clean
	mkdir -p build
	go build -o build/deliveryd ./cmd/deliveryd
	go build -o build/deliverycli ./cmd/deliverycli
	@echo "====================================================\n==================Build Successful==================\n===================================================="

install:
	go install $(BUILD_FLAGS) ./cmd/deliveryd
	go install $(BUILD_FLAGS) ./cmd/deliverycli

contracts:
	abigen --abi=contracts/rootchain/rootchain.abi --pkg=rootchain --out=contracts/rootchain/rootchain.go
	abigen --abi=contracts/stakemanager/stakemanager.abi --pkg=stakemanager --out=contracts/stakemanager/stakemanager.go
	abigen --abi=contracts/slashmanager/slashmanager.abi --pkg=slashmanager --out=contracts/slashmanager/slashmanager.go
	abigen --abi=contracts/statereceiver/statereceiver.abi --pkg=statereceiver --out=contracts/statereceiver/statereceiver.go
	abigen --abi=contracts/statesender/statesender.abi --pkg=statesender --out=contracts/statesender/statesender.go
	abigen --abi=contracts/stakinginfo/stakinginfo.abi --pkg=stakinginfo --out=contracts/stakinginfo/stakinginfo.go
	abigen --abi=contracts/validatorset/validatorset.abi --pkg=validatorset --out=contracts/validatorset/validatorset.go
	abigen --abi=contracts/erc20/erc20.abi --pkg=erc20 --out=contracts/erc20/erc20.go
#
# Code quality
#

LINT_COMMAND := $(shell command -v golangci-lint 2> /dev/null)
lint:
ifndef LINT_COMMAND
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.53.3
endif
	golangci-lint run --config ./.golangci.yml

#
# docker commands
#

build-docker:
	@echo Fetching latest tag: $(LATEST_GIT_TAG)
	git checkout $(LATEST_GIT_TAG)
	docker build -t "delivery:$(LATEST_GIT_TAG)" -f docker/Dockerfile .

build-docker-develop:
	docker build -t "delivery:develop" -f docker/Dockerfile.develop .

.PHONY: contracts build
