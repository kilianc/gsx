# Pinned toolchain for the README demo animation.
#
# Deliberately a second image rather than more layers on tools/Dockerfile. That
# one is built by CI on every run to test the extension, and a Rust build stage
# would add minutes to a job that has no use for it. This one is built only when
# someone re-records the demo.
#
# Build from the repository root:
#
#   docker build -f tools/demo.Dockerfile -t gsx-demo .
#
# Targets that use it:
#
#   make demo IN=recording.mov    convert a screen recording into assets/gsx-demo.gif

# gifski ships no arm64 Linux binary, so it is built from source. The stage is
# discarded and only the binary is copied forward, which keeps the Rust
# toolchain out of the image that actually runs.
FROM rust:1.88-slim-bookworm AS gifski
RUN cargo install gifski --version 1.34.0 --locked --root /opt/gifski

FROM debian:bookworm-slim

# ffmpeg decodes the recording and resamples it; gifski does the quantising.
# Splitting it that way is the point of the image: ffmpeg's one global 256-colour
# palette is what makes a screen recording look muddy, and gifski picks a palette
# per frame instead.
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates ffmpeg \
 && rm -rf /var/lib/apt/lists/*

COPY --from=gifski /opt/gifski/bin/gifski /usr/local/bin/gifski

WORKDIR /work
