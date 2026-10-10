# The trial runner image: the claude and codex CLIs at pinned versions, Node LTS, git, flow-eval, and
# two JDKs. gymie compiles and tests on a JDK 25 toolchain (JavaLanguageVersion.of(25)), but its
# Gradle build itself runs on JDK 21: detekt 1.23.8 in its buildSrc fails on a JDK 25 runtime
# (measured: `:buildSrc:detekt` fails with "> 25.0.4"). So JAVA_HOME is JDK 21, and JDK 25 sits in
# /usr/lib/jvm, where Gradle's toolchain auto-detection finds it. procps provides the `ps` gymie's
# buildSrc tests call. libgl1, libegl1 and libfontconfig1 are what skiko's native library links
# against: without them every Compose desktop UI test in gymie-frontend fails to load it
# (measured: `libGL.so.1: cannot open shared object file`). curl is what the Muse Code launcher
# downloads with during the one-time setup (docs/muse-code.md); bash and sha256sum, which
# it also needs, are in the base image. Trials run as root with HOME=/root,
# whose ~/.gradle is the trial's own; the sandbox mounts the per-repo cache volume read-only at
# /gradle-ro (GRADLE_RO_DEP_CACHE).
# The image also carries flow's measure-visual-properties.sh, built from the agents repository at
# exactly prompts/PIN (decision visual-measure-tool-in-runner). Upstream, that script is a shim that
# execs `flow-guard measure-visual-properties` built from its checkout, so the image carries that
# binary, built in the measure stage, and a one-line shim on PATH. The `agents` build context holds
# agents.tar, a `git archive` of the pin; the build refuses a tar of any other commit, so neither a
# working tree nor a stale archive can stand in for the pin.
# Build from the repository root:
#   agents=$(mktemp -d)
#   git -C <agents checkout> archive -o "$agents/agents.tar" "$(cat prompts/PIN)" stats
#   docker build -f docker/runner.Dockerfile --build-context agents="$agents" -t flow-eval-runner .
FROM golang:1.27.2-trixie AS measure
COPY prompts/PIN /pin
RUN --mount=type=bind,from=agents,target=/agents \
    if [ "$(git get-tar-commit-id < /agents/agents.tar)" != "$(cat /pin)" ]; then \
      echo "agents.tar is not a git archive of prompts/PIN $(cat /pin)" >&2; exit 1; \
    fi \
 && mkdir /src && tar -xf /agents/agents.tar -C /src \
 && cd /src/stats && CGO_ENABLED=0 go build -trimpath -o /out/flow-guard ./cmd/flow-guard

FROM golang:1.27.2-trixie AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/flow-eval ./cmd/flow-eval

FROM eclipse-temurin:21.0.12.1_1-jdk-noble AS jdk21
FROM eclipse-temurin:25.0.4.1_1-jdk-noble AS jdk25

FROM node:24.21.0-trixie-slim
ENV JAVA_HOME=/opt/java/openjdk
ENV PATH="${JAVA_HOME}/bin:${PATH}"
COPY --from=jdk21 /opt/java/openjdk /opt/java/openjdk
COPY --from=jdk25 /opt/java/openjdk /usr/lib/jvm/temurin-25
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl git procps \
      libgl1 libegl1 libfontconfig1 \
 && rm -rf /var/lib/apt/lists/*
RUN npm install -g @anthropic-ai/claude-code@2.1.286 @openai/codex@0.162.0 \
 && npm cache clean --force
COPY --from=build /out/flow-eval /usr/local/bin/flow-eval
COPY --from=measure /out/flow-guard /usr/local/bin/flow-guard
RUN printf '#!/bin/sh\nexec flow-guard measure-visual-properties "$@"\n' > /usr/local/bin/measure-visual-properties.sh \
 && chmod 755 /usr/local/bin/measure-visual-properties.sh
WORKDIR /work
