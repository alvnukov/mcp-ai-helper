---
id: task-registry-init-tool
title: Add MCP-only task registry initialization tool
status: done
priority: high
model_level: medium
tags:
    - tasks
    - registry
    - obsidian
    - mcp
    - setup
created_at: "2026-05-27T20:52:56.181119Z"
updated_at: "2026-05-27T20:52:56.202691Z"
---

## Body

Add an MCP-only setup tool so an LLM can initialize a repo for Obsidian-backed tasks without direct shell/filesystem fallback. The tool must support dry-run, create the configured repo-local task directory, optionally write repo .mcp-ai-helper.yaml, validate the initialized backend, and return next_call: task_current.
