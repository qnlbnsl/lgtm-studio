# syntax=docker/dockerfile:1
# The server image. The stage build compiles one static program. The stage runtime holds that
# program on a base image with no shell, and runs it as an ordinary user. Build it from the
# repository root:
#   docker build -f deploy/server.Dockerfile -t lgtm-server:local .
# The allow-list next to this file keeps the state folder and every other path out of the build.

FROM golang:1.26.11-alpine@sha256:ecb67f987aa1a57794c91d19ade7ae0209f2f0f5496aaef9e6ae85ddf92d02ab AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags='-s -w' -o /out/lgtm-server ./src/server/cmd/lgtm-server

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS runtime
COPY --from=build /out/lgtm-server /usr/local/bin/lgtm-server
USER 10001:10001
# Running the image with no arguments starts the server.
ENTRYPOINT ["/usr/local/bin/lgtm-server"]
