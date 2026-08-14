---
id: workflow-preview-must-respect-repo-deny
title: Workflow preview должен уважать repo deny
status: done
priority: low
model_level: low
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - pipeline
    - policy
    - deny
acceptance_criteria:
    - preview строится только после успешного pipelineRunnerForRepo (включая deny-проверку)
    - deny отдаёт ошибку deny, а не preview
    - тест
created_at: "2026-08-14T21:18:50.509115Z"
updated_at: "2026-08-14T21:38:47.115975Z"
---

## Body

pipeline_tools.go:93-124 — preview собирается до pipelineRunnerForRepo: репо, запрещающее run action=workflow, всё равно получает превью шагов/коммитов.

## Acceptance Criteria

- preview строится только после успешного pipelineRunnerForRepo (включая deny-проверку)
- deny отдаёт ошибку deny, а не preview
- тест
