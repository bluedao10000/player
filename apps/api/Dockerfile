FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/api . && CGO_ENABLED=0 go build -trimpath -o /out/knowledge-sync ./cmd/knowledge-sync

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /out/* /usr/local/bin/
USER 65534:65534
CMD ["api"]
