FROM scratch
# Copy our static executable.
COPY bin/ps-otu-sqlsync64 /go/bin/ps-otu-sqlsync
# Run sync service
ENTRYPOINT ["/go/bin/ps-otu-sqlsync"]