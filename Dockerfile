FROM gcr.io/cloud-marketplace/google/ubuntu2204:latest

# minimal dependencies for getting bazel/bazelisk
# tzdata needed for some conformance tests.
RUN apt-get update && apt-get upgrade -y && \
     DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      build-essential \
      bash \
      ca-certificates \
      git \
      libssl-dev \
      make \
      pkg-config \
      python3 \
      tzdata \
      unzip \
      wget \
      zip \
      zlib1g-dev \
      default-jdk-headless && \
    apt-get clean

ARG BAZELISK_RELEASE="https://github.com/bazelbuild/bazelisk/releases/download/v1.27.0/bazelisk-amd64.deb"
ARG BAZELISK_CHKSUM="sha256:d8b00ea975c823e15263c80200ac42979e17368547fbff4ab177af035badfa83"

# cloud build doesn't support --checksum arg to add
ADD ${BAZELISK_RELEASE} /tmp/bazelisk.deb

RUN apt-get install /tmp/bazelisk.deb

RUN mkdir -p /workspace
RUN mkdir -p /bazel

# note: /usr/bin/bazel is also a symlink to bazelisk
ENTRYPOINT ["/usr/bin/bazelisk"]
