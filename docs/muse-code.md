# Muse Code setup

A muse-code trial runs Meta's Muse Code CLI from the Docker volume `flow-eval-muse`, which this
one-time setup fills: the launcher and the Linux binary it downloads under `/muse/bin`, and the
account login under `/muse/config`. Each step signs in with a code approved in your browser. Build
the runner image first (it carries the `curl` the launcher downloads with), then run, from any
directory:

```bash
# 1. Install the launcher; it downloads the binary, asking you to sign in.
docker run -it --rm -v flow-eval-muse:/muse flow-eval-runner bash -c \
  'mkdir -p /muse/bin /muse/config && curl -fsSL https://dev.meta.ai/install.sh | MUSE_INSTALL_DIR=/muse/bin bash'

# 2. Log the CLI in; the login is stored in /muse/config/muse/auth.json.
docker run -it --rm -v flow-eval-muse:/muse -e XDG_CONFIG_HOME=/muse/config flow-eval-runner \
  /muse/bin/muse login

# 3. Check: prints the binary's version, then "logged in".
docker run --rm -v flow-eval-muse:/muse flow-eval-runner sh -c \
  '/muse/bin/muse-bin-$(cat /muse/bin/.muse-version) --version && test -s /muse/config/muse/auth.json && echo logged in'
```

Trials run the binary `/muse/bin/.muse-version` names, mounted read-only, never the launcher, so a
trial neither updates nor downloads it; repeat step 1 to move to a newer release. The login is
mounted read-only: each trial copies it into its own config home, so nothing one trial writes
reaches another, and a token the CLI refreshes during a trial is not saved back. If trials start
failing on auth, re-run step 2. A run whose volume does not exist, or lacks the binary or the
login, exits 2 naming this setup before any trial starts.
