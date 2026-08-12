# Pinned toolchain for the VS Code extension.
#
# The extension is the only part of this repository that needs Node. Keeping it
# in an image means contributors never install a JS toolchain to work on GSX,
# and the versions that build a release are recorded here rather than in
# whatever happened to be on someone's machine.
#
# Build from the repository root so the extension manifest is in context:
#
#   docker build -f tools/Dockerfile -t gsx-tools .
#
# Targets that use it:
#
#   make grammar-test     tokenize fixtures with the real TextMate engine
#   make extension-test   unit tests
#   make extension-check  typecheck
#   make vsix             package the extension
FROM node:22.12.0-bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates git \
 && rm -rf /var/lib/apt/lists/*

# vsce is installed globally, deliberately: as a devDependency of the extension
# it makes `npm install` crash with "Exit handler never called!" after eight
# minutes, reproducibly, on both macOS Docker and GitHub's Ubuntu runners.
# Removing it from the project tree takes that install from 8 minutes of failure
# to 5 seconds of success.
RUN npm install -g --ignore-scripts --no-audit --no-fund @vscode/vsce@3.2.2

# vscode-textmate and vscode-oniguruma are the engine VS Code itself uses to
# tokenize, so a grammar test here reflects what an editor will actually do.
RUN mkdir -p /opt/grammar && cd /opt/grammar \
 && npm install --ignore-scripts --no-package-lock --no-audit --no-fund \
      vscode-textmate@9.2.0 \
      vscode-oniguruma@2.0.1

# Just enough to typecheck the extension: the compiler, the VS Code API types,
# and the two runtime packages the sources import.
#
# Deliberately not the full devDependency tree — webpack and vsce drag in native
# modules that make an image build slow and flaky, and none of it is needed to
# check types. Packaging (`make vsix`) installs the rest at run time.
#
# This matters because without @types/vscode present tsc reports a missing type
# library and never checks the source at all, so a typecheck that "passes" for
# that reason is worthless.
WORKDIR /opt/ext
RUN npm install --ignore-scripts --no-package-lock --no-audit --no-fund \
      typescript@5.7.2 \
      @types/vscode@1.95.0 \
      @types/node@22.10.2 \
      vscode-languageclient@9.0.1 \
      lookpath@1.2.2

COPY vscode/gsx-vscode/tsconfig.json ./

ENV GSX_NODE_MODULES=/opt/grammar/node_modules \
    NPM_CONFIG_UPDATE_NOTIFIER=false \
    NPM_CONFIG_FUND=false

WORKDIR /work
