IMAGE_REPO ?=

ifeq (${PLATFORMS}, )
	export PLATFORMS="linux/arm64,linux/amd64"
endif

ifeq (${DOCKER_IMG_BUILD}, load)
	export DOCKER_ARG="--load"
	export PLATFORMS="local"
else ifeq (${DOCKER_IMG_BUILD}, cache)
	# if specified, image will only be available in the build cache, it won't be pushed or loaded
else
	export PUSH_ARG="--push"
endif

all: build-controller build-tarter build-hook

build-controller:
	@echo "Building controller"
	go build -o bin/controller ./cmd/controller

build-tarter:
	@echo "Building tarter"
	go build -o bin/tarter ./cmd/tarter

build-hook:
	@echo "Building hook"
	go build -o bin/hook ./cmd/hook

# Run go fmt against code
fmt:
	go fmt ./...

docker: docker-buildx docker-hook docker-controller

docker-buildx:
	@echo "Checkin existing buildx platforms"
	export DOCKER_CLI_EXPERIMENTAL=enabled ;\
	export DOCKER_BUILDKIT=1
	@if ! docker buildx ls | grep -q container-builder; then\
		docker buildx create --platform ${PLATFORMS} --name container-builder --use;\
	fi
	
docker-hook:
	@echo "Building hook docker image"
	docker buildx build --platform ${PLATFORMS} \
	--build-arg TARGET_APP=hook \
	-t "${IMAGE_REPO}/hook:${VERSION}" \
	-f Dockerfile \
	. ${PUSH_ARG}

docker-controller:
	@echo "Building controller docker image"
	docker buildx build --platform ${PLATFORMS} \
	--build-arg TARGET_APP=controller \
	-t "${IMAGE_REPO}/controller:${VERSION}" \
	-f Dockerfile \
	. ${PUSH_ARG}