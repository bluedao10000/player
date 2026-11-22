# Build from this release's apps/image runtime for a matching restore binary.
ARG IMAGE_RUNTIME=folio-image:local
FROM ${IMAGE_RUNTIME}
USER root
RUN apt-get update && apt-get install -y --no-install-recommends rsync openssh-client netcat-openbsd jq \
    && rm -rf /var/lib/apt/lists/* \
    && (getent group 10001 >/dev/null || groupadd -g 10001 image-backup) \
    && (getent passwd 10001 >/dev/null || useradd -M -d /tmp -u 10001 -g 10001 image-backup)
COPY --chmod=755 pull-verify.sh /usr/local/bin/image-pull-verify
USER 10001:10001
ENTRYPOINT ["image-pull-verify"]
