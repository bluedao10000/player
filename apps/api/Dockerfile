FROM golang:1.25-alpine AS build
WORKDIR /src
COPY database/go.mod database/go.sum ./database/
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
WORKDIR /src/apps/api
RUN go mod download
COPY database /src/database
COPY apps/api /src/apps/api
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /out/* /usr/local/bin/
RUN mkdir -p /var/lib/knowledge/archive && chown 65534:65534 /var/lib/knowledge/archive
USER 65534:65534
CMD ["api"]
