ARG BASE_IMAGE=golang:1.26.4-bookworm
FROM ${BASE_IMAGE}
COPY --chmod=0555 build run /opt/judge/
USER 65532:65532
WORKDIR /sandbox/artifact
ENTRYPOINT ["/opt/judge/run"]
