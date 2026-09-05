ARG BASE_IMAGE=golang:1.26.4-bookworm
FROM ${BASE_IMAGE} AS compiler
COPY --chmod=0555 build /opt/judge/build
COPY runner.go /opt/judge/runner.go
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
RUN ["/usr/local/go/bin/go", "build", "-trimpath", "-ldflags=-buildid=", "-o", "/opt/judge/run", "/opt/judge/runner.go"]
USER 65532:65532
WORKDIR /sandbox/artifact
ENTRYPOINT ["/opt/judge/run"]

FROM scratch AS runtime
COPY --from=compiler /opt/judge/run /opt/judge/run
USER 65532:65532
WORKDIR /sandbox/artifact
ENTRYPOINT ["/opt/judge/run"]
