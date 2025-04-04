FROM --platform=$BUILDPLATFORM golang:1.24 AS build
WORKDIR /builder

# Make it runnable on a distroless image/without libc
ENV CGO_ENABLED=0

# TARGET_OS can be "linux", "darwin",
# TARGET_ARCH can be "amd64", "arm64", and "arm"
ARG TARGET_APP TARGET_OS TARGET_ARCH VERSION=0.0.0 COMMIT_SHA=dev BUILD_DATE=dev

COPY go.mod go.sum ./
RUN go mod download

RUN --mount=target=. \
    export GOOS=${TARGET_OS} GOARCH=${TARGET_ARCH} && \
    go build -trimpath \
      -ldflags "-s -w -X github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version.Version=${VERSION} -X github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version.BuildDate=${BUILD_DATE} -X github.com/tomtom-international/macos-actions-runner-controller/pkg/core/version.GitCommit=${COMMIT_SHA}" \
      -o /out/${TARGET_APP} ./cmd/${TARGET_APP}

FROM gcr.io/distroless/static:nonroot
WORKDIR /

ARG TARGET_APP

COPY --from=build /out/${TARGET_APP} /app
ENTRYPOINT [ "/app" ]