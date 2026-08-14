# Release process

Releases are tag-driven. A pushed semantic version tag builds immutable
cross-platform archives, publishes a GitHub Release with checksums, and only
then updates `alvnukov/homebrew-tap`.

## One-time repository setup

Create the GitHub Actions secret `HOMEBREW_TAP_TOKEN` in
`alvnukov/mcp-ai-helper`. Use a fine-grained token with read/write Contents
access to `alvnukov/homebrew-tap`. A missing secret fails the Homebrew job;
it is never treated as a successful partial release.

Both repositories must remain public because the generated formula downloads
GitHub Release assets without credentials.

## Release contract

1. Release from `main` after `make quality`, `make lint`, and actionlint pass.
2. Use an annotated tag named `vMAJOR.MINOR.PATCH`.
3. The workflow strips the leading `v` and injects that exact version into
   `mcp-ai-helper --version` and the MCP server handshake and `health` version.
4. The release is complete only when the GitHub Release contains all platform
   archives plus `checksums.txt`, and `Formula/mcp-ai-helper.rb` in the tap
   references the same version and archive hashes.

For example:

```sh
git tag -a v1.0.0 -m "mcp-ai-helper v1.0.0"
git push origin v1.0.0
```

Do not reuse or move a published tag. Fix forward with a new patch version.
