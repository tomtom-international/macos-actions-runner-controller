IMAGE_REPO ?=
VERSION ?= 0.0.0
COMMIT_SHA = $(shell git rev-parse HEAD)
BUILD_DATE = $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
BUILD_PLATFORMS ?= linux/arm64 darwin/arm64

GO_BUILD_LDFLAGS = "-s -w -X github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version.Version=${VERSION} -X github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version.BuildDate=${BUILD_DATE} -X github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version.GitCommit=${COMMIT_SHA}"

ifeq (${PLATFORMS}, )
	export PLATFORMS="linux/arm64,linux/amd64"
endif

ifeq (${DOCKER_IMG_BUILD}, load)
	export PUSH_ARG=--load
else ifeq (${DOCKER_IMG_BUILD}, cache)
	# if specified, image will only be available in the build cache, it won't be pushed or loaded
else
	export PLATFORMS_ARG=--platform ${PLATFORMS}
	export PUSH_ARG=--push
endif

build: build-controller build-tarter build-hook build-metrics-aggregator

build-controller:
	@echo "Building controller"
	go build -ldflags ${GO_BUILD_LDFLAGS} \
		-o bin/controller ./cmd/controller

build-metrics-aggregator:
	@echo "Building metrics-aggregator"
	go build -ldflags ${GO_BUILD_LDFLAGS} \
		-o bin/metrics-aggregator ./cmd/metrics-aggregator

build-tarter:
	@echo "Building tarter"
	go build -ldflags ${GO_BUILD_LDFLAGS} \
		-o bin/tarter ./cmd/tarter

build-hook:
	@echo "Building hook"
	go build -ldflags ${GO_BUILD_LDFLAGS} \
		-o bin/hook ./cmd/hook

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

lint:
	golangci-lint run ./...

docker: docker-hook docker-controller

docker-buildx:
	@echo "Checkin existing buildx platforms"
	export DOCKER_CLI_EXPERIMENTAL=enabled ;\
	export DOCKER_BUILDKIT=1
	@if ! docker buildx ls | grep -q container-builder; then\
		docker buildx create --platform ${PLATFORMS} --name container-builder --use;\
	fi

docker-hook:
	@echo "Building hook docker image"
	docker buildx build ${PLATFORMS_ARG} \
	--build-arg TARGET_APP=hook \
	--build-arg VERSION=${VERSION} \
	--build-arg COMMIT_SHA=${COMMIT_SHA} \
	--build-arg BUILD_DATE=${BUILD_DATE} \
	-t "${IMAGE_REPO}/hook:${VERSION}" \
	-f Dockerfile \
	. ${PUSH_ARG}

docker-controller:
	@echo "Building controller docker image"
	docker buildx build ${PLATFORMS_ARG} \
	--build-arg TARGET_APP=controller \
	--build-arg VERSION=${VERSION} \
	--build-arg COMMIT_SHA=${COMMIT_SHA} \
	--build-arg BUILD_DATE=${BUILD_DATE} \
	-t "${IMAGE_REPO}/controller:${VERSION}" \
	-f Dockerfile \
	. ${PUSH_ARG}

# Create release artifacts
release-prep:
	@echo "Preparing release artifacts"
	@mkdir -p release
	@echo "Release artifacts will be created in ./release directory"

release-tarter:
	@echo "Prepare tarter release artifacts"
	@for platform in $(BUILD_PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		echo "Building tarter. OS: $$os, ARCH: $$arch"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags ${GO_BUILD_LDFLAGS} \
			-o release/macos-actions-runner-tarter-$$os-$$arch ./cmd/tarter; \
	done

release-metrics-aggregator:
	@echo "Prepare metrics-aggregator release artifacts"
	@for platform in $(BUILD_PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		echo "Building metrics-aggregator. OS: $$os, ARCH: $$arch"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags ${GO_BUILD_LDFLAGS} \
			-o release/macos-actions-runner-metrics-aggregator-$$os-$$arch ./cmd/metrics-aggregator; \
	done

release-checksums:
	@echo "Generating checksums for release artifacts"
	@cd release && sha256sum * > macos-actions-runner-checksums.txt

release: release-prep release-tarter release-metrics-aggregator release-checksums

# Clean release artifacts
clean-release:
	@echo "Cleaning release artifacts"
	rm -rf release

.PHONY: release-prep release-tarter release-metrics-aggregator release-checksums clean-release
