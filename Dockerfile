FROM golang:1.20-bullseye as base
ARG APP_BUID_VERSION
ARG CI_COMMIT_SHA
ARG APP_GIT_HASH
RUN adduser \
  --disabled-password \
  --gecos "" \
  --home "/nonexistent" \
  --shell "/sbin/nologin" \
  --no-create-home \
  --uid 65532 \
  small-user

WORKDIR $GOPATH/src/smallest-golang/app/

COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV LDFLAGS="-X main.version=${APP_BUID_VERSION} -X main.commit=${APP_GIT_HASH} -X main.hash=${CI_COMMIT_SHA}"
RUN go build -ldflags="${LDFLAGS}"  -o /exporter cmd/cosmos-validators-exporter.go

FROM scratch
ARG CI_COMMIT_SHA
ARG APP_BUID_VERSION
LABEL git-commit=$CI_COMMIT_SHA
LABEL build-version=$APP_BUID_VERSION
COPY --from=base /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=base /etc/passwd /etc/passwd
COPY --from=base /etc/group /etc/group

COPY --from=base /exporter .
COPY config.example.toml config.toml

USER small-user:small-user

EXPOSE 9560

CMD ["./exporter", "--config", "config.toml"]