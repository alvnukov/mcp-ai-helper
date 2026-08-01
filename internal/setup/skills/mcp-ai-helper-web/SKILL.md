---
name: mcp-ai-helper-web
description: Research external sources through mcp-ai-helper with the search, fetch, find, read protocol, so a conclusion rests on a retrieved document rather than on a result snippet. Use whenever an answer needs documentation, an API contract, a changelog, a standard, a version fact, or anything else that is not already in the repository, and whenever a fetch is blocked, truncated, or returns less than the question needs.
---

# Web research through mcp-ai-helper

`web_search` finds candidates, `web_fetch` retrieves a document,
`fetched_doc_find` locates the passage, `fetched_doc_read` returns it. Skipping a
step is how a search snippet ends up cited as a fact.

## Protocol

1. Call `web_search` for compact hits. A hit is a candidate, never evidence: its
   snippet is written by the index, not by the page.
2. Choose a few URLs and call `web_fetch` for each. It returns doc_id, URL
   metadata, hashes, cache status, completeness and diagnostics — not page
   content. The doc_id is what makes the document quotable later without
   fetching it again.
3. Call `fetched_doc_find` on the doc_id to search the complete normalized
   document and get bounded snippets with offsets.
4. Call `fetched_doc_read` only after a find or a known offset, and only for a
   bounded fragment around the evidence. Never request a whole page.

## Evidence

Cite by doc_id, URL or source, offsets, and the snippet actually read.

Check the completeness and diagnostics `web_fetch` reports before treating a
document as whole. A truncated fetch that reads as complete is the same class of
error as a truncated command log, and it fails the same way: a confident answer
built on the part that happened to arrive.

If a fetch was blocked, incomplete, or holds nothing that answers the question,
say insufficient_evidence. An answer assembled from search snippets is the
failure this protocol exists to prevent, and it is worse than no answer because
it looks like one.

## Providers

For Google, call `web_search` with provider=google_cse and the configured
web_policy.google_cse_id plus google_api_key_env or google_api_key. Do not scrape
Google HTML results.

If `web_search` or `web_fetch` is not in `tool_manifest`, the web layer is off
for this server. Report a surface mismatch rather than substituting a generic
browser, search, or fetch tool.
