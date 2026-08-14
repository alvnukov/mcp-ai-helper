---
id: websearch-diagnostics-must-not-leak-api-key
title: Диагностика websearch не должна утекать API-ключ
status: done
priority: high
model_level: medium
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - security
    - secrets
    - websearch
acceptance_criteria:
    - в diagnostics/логах не появляется key= из URL запроса ни при транспортной ошибке, ни при иных ошибках
    - тест на transport-ошибку с ключом в конфиге
    - замаскированный текст проходит через секрет-маску хелпера
created_at: "2026-08-14T21:18:50.493872Z"
updated_at: "2026-08-14T21:27:48.59029Z"
---

## Body

websearch/search.go:164,219-229 — ключ Google CSE в query-string; при транспортной ошибке search_failed embed'ит err.Error() (*url.Error содержит полный URL с key=…). Секрет уходит в модель-видимый вывод и логи.

## Acceptance Criteria

- в diagnostics/логах не появляется key= из URL запроса ни при транспортной ошибке, ни при иных ошибках
- тест на transport-ошибку с ключом в конфиге
- замаскированный текст проходит через секрет-маску хелпера
