---
id: workflow-same-file-must-normalize-paths
title: Workflow-шаги должны нормализовать пути одного файла
status: done
priority: high
model_level: medium
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - pipeline
    - workflow
    - concurrency
    - paths
acceptance_criteria:
    - ключи same-file сериализации, fileLocks и hash-chain — на очищенном пути (filepath.Clean), a.go и ./a.go это один файл
    - ChangedFiles содержит нормализованный путь один раз
    - тест на два шага с разными написаниями одного файла выполняются последовательно без lost update
created_at: "2026-08-14T21:18:50.501425Z"
updated_at: "2026-08-14T21:46:18.272289Z"
---

## Body

pipeline.go:681 — stepFilePath ключует сериализацию/локи/hash-chain на сыром args.path: a.go и ./a.go дают параллельную запись в один файл (lost update), оба написания попадают в ChangedFiles.

## Acceptance Criteria

- ключи same-file сериализации, fileLocks и hash-chain — на очищенном пути (filepath.Clean), a.go и ./a.go это один файл
- ChangedFiles содержит нормализованный путь один раз
- тест на два шага с разными написаниями одного файла выполняются последовательно без lost update
