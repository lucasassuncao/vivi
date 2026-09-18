# Built rather than bind-mounted: a bind mount is resolved by the daemon, which
# under docker-outside-of-docker is the host's and cannot see /workspaces. A
# build context is read by the client, so one compose file works from both.
FROM hashicorp/vault:1.20.4

COPY seed.sh /seed.sh

# The CR strip survives a CRLF checkout. It runs here and not in a RUN because
# this image runs as an unprivileged user that cannot rewrite a file in /.
ENTRYPOINT ["/bin/sh", "-c", "sed -e 's/\r$//' /seed.sh > /tmp/seed.sh && exec sh /tmp/seed.sh"]
