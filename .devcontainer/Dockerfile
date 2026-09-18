FROM ubuntu:24.04

# Install basic tools and apply security updates.
#
# socat and procps are not incidental: vault-forward.sh needs socat to put the
# host's Vault on this container's loopback, and pgrep to notice it has already
# done so. The docker-outside-of-docker feature happens to install socat too,
# for its own socket proxy, but the forwarder should not depend on a side
# effect of another feature's implementation.
RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get install -y --no-install-recommends \
    curl \
    wget \
    vim \
    nano \
    less \
    build-essential \
    ca-certificates \
    strace \
    lsof \
    iproute2 \
    dnsutils \
    htop \
    ripgrep \
    plocate \
    socat \
    procps \
    gnupg && \
    rm -rf /var/lib/apt/lists/*
