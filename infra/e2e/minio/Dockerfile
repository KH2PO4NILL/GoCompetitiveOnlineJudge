FROM golang:1.26.4-bookworm AS build
ARG GOPROXY=https://proxy.golang.org,direct
# Official source-only release. Go's checksum database verifies the pinned module.
# https://github.com/minio/minio/releases/tag/RELEASE.2025-10-15T17-29-55Z
RUN CGO_ENABLED=0 GOBIN=/out go install github.com/minio/minio@RELEASE.2025-10-15T17-29-55Z
COPY healthcheck.go /healthcheck.go
RUN CGO_ENABLED=0 go build -trimpath -o /out/healthcheck /healthcheck.go
RUN mkdir -p /runtime/data /runtime/tmp && chown -R 65532:65532 /runtime && chmod 1777 /runtime/tmp

FROM scratch
COPY --from=build /out/minio /minio
COPY --from=build /out/healthcheck /healthcheck
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build --chown=65532:65532 /runtime/data /data
COPY --from=build --chown=65532:65532 /runtime/tmp /tmp
USER 65532:65532
EXPOSE 9000
ENTRYPOINT ["/minio"]
CMD ["server", "/data"]
