# Pinned, hermetic CLDR generation toolchain for gofluent.
#
# Why this exists: the generated cldr/* tables and the golden Intl fixtures must
# describe the SAME CLDR release. The host's Node bundles whatever ICU/CLDR it was
# built with (e.g. Node 22.15 -> ICU 76 -> CLDR 46), which is not something we can
# rely on. So we pin BOTH knobs here:
#   * the Node image (digest-pinned) fixes the ICU/CLDR used by Intl.* golden fixtures;
#   * the npm cldr-* packages (gen/package.json) fix the CLDR JSON the Go generators read.
# Both are CLDR 46, so `go generate ./...` and the Intl fixture dumps agree by
# construction — no host Node involved.
FROM node:22.15.0-bookworm@sha256:a1f1274dadd49738bcd4cf552af43354bb781a7e9e3bc984cfeedc55aba2ddd8

# Go toolchain (matches the module's go directive).
ARG GO_VERSION=1.26.0
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}" \
    GOTOOLCHAIN=local

# Pin the CLDR data release. gen/package.json is the single source of truth.
WORKDIR /opt/cldr
COPY package.json ./
RUN npm install --no-audit --no-fund --loglevel=error

# Generators read CLDR JSON from $CLDR_DATA; Node fixture scripts resolve cldr-* via NODE_PATH.
ENV CLDR_DATA=/opt/cldr/node_modules \
    NODE_PATH=/opt/cldr/node_modules

WORKDIR /work
