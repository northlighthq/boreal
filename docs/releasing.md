# Releasing

Write the changes under `## Unreleased` in `CHANGELOG.md`, then run the **Release** workflow
from Actions with the version (`0.1.1-beta.5`). It tests on the three systems, scans the
dependencies, packages Windows, macOS and Linux, signs the checksums and publishes the
release. Tick *dry run* to build everything without publishing.
