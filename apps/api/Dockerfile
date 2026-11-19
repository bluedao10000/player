FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/api . && CGO_ENABLED=0 go build -trimpath -o /out/knowledge-sync ./cmd/knowledge-sync && CGO_ENABLED=0 go build -trimpath -o /out/knowledge ./cmd/knowledge && CGO_ENABLED=0 go build -trimpath -o /out/media ./cmd/media

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /out/* /usr/local/bin/
RUN mkdir -p /var/lib/knowledge/archive && chown 65534:65534 /var/lib/knowledge/archive
USER 65534:65534
CMD ["api"]
