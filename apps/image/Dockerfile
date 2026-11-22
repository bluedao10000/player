# syntax=docker/dockerfile:1
FROM rust:1.95-trixie AS toolchain
RUN apt-get update && apt-get install -y --no-install-recommends libvips-dev libheif-plugin-aomenc libheif-plugin-dav1d && rm -rf /var/lib/apt/lists/*
WORKDIR /src

FROM toolchain AS test
RUN rustup component add rustfmt clippy
COPY . .
RUN --mount=type=cache,target=/usr/local/cargo/registry --mount=type=cache,target=/src/target \
    cargo fmt --all -- --check && cargo clippy --locked --all-targets -- -D warnings && cargo test --locked

FROM toolchain AS build
COPY . .
RUN --mount=type=cache,target=/usr/local/cargo/registry --mount=type=cache,target=/src/target \
    cargo build --release --locked && install -Dm755 target/release/folio-image /out/folio-image

FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl libvips42t64 libheif-plugin-aomenc libheif-plugin-dav1d sqlite3 && rm -rf /var/lib/apt/lists/* \
    && mkdir /data && chown 10001:10001 /data
COPY --from=build /out/folio-image /usr/local/bin/
COPY ops/backup.sh /usr/local/bin/folio-image-backup
ENV IMAGE_BIND=0.0.0.0:14580 IMAGE_DATA_DIR=/data
USER 10001:10001
EXPOSE 14580
HEALTHCHECK --interval=15s --timeout=3s CMD curl -fsS http://127.0.0.1:14580/readyz || exit 1
ENTRYPOINT ["folio-image"]
