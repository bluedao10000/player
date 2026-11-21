FROM golang:1.25-alpine AS build
WORKDIR /src
COPY database/go.mod database/go.sum ./database/
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
COPY apps/sync/go.mod apps/sync/go.sum ./apps/sync/
WORKDIR /src/apps/sync
RUN go mod download
COPY database /src/database
COPY apps/api /src/apps/api
COPY apps/sync /src/apps/sync
ENV CGO_ENABLED=0
RUN go build -trimpath -o /out/knowledge-sync ./cmd/sync
RUN go build -trimpath -o /out/ ./cmd/backup-report ./cmd/reader-release ./cmd/knowledge

FROM alpine:3.22
RUN apk add --no-cache ca-certificates openssh-client && mkdir -p /var/lib/knowledge/archive && chown 65534:65534 /var/lib/knowledge/archive
COPY --from=build /out/ /usr/local/bin/
USER 65534:65534
CMD ["knowledge-sync"]
