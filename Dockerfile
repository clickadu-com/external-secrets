# Build stage
ARG GolangVersion=1.26.2-202604141903
FROM nexus.adsrv.wtf/base/golang:${GolangVersion} AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG GOPRIVATE

ENV CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GOPRIVATE=${GOPRIVATE}

WORKDIR /app

COPY . /app/

RUN --mount=type=cache,target=${GOPATH},mode=0777,uid=10000,gid=10000 \
    go build -o bin/external-secrets main.go

# Final stage
FROM gcr.io/distroless/static@sha256:47b2d72ff90843eb8a768b5c2f89b40741843b639d065b9b937b07cd59b479c6 AS app
COPY --from=builder /app/bin/external-secrets /bin/external-secrets

USER 65534
ENTRYPOINT ["/bin/external-secrets"]
