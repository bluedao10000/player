FROM scratch
COPY backup-report /backup-report
ENTRYPOINT ["/backup-report"]
