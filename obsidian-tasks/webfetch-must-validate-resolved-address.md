---
id: webfetch-must-validate-resolved-address
title: webfetch должен проверять адрес фактического соединения (SSRF)
status: done
priority: critical
model_level: high
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - security
    - ssrf
    - webfetch
    - web
acceptance_criteria:
    - проверка публичности адреса происходит в момент диала (DialContext/Control-хук) по резолвеному IP, а не только по строке host
    - redirect на внутренний адрес блокируется тем же механизмом
    - allowed_hosts в политике сохраняет семантику явного доверия
    - 'тесты: loopback/приватный IP блокируется, публичный хост проходит'
created_at: "2026-08-14T21:18:50.492474Z"
updated_at: "2026-08-14T21:53:15.484455Z"
---

## Body

webfetch/fetch.go:223-247 — validateURL проверяет публичность только для literal-IP из net.ParseIP. DNS-имя, резолвящееся в 127.0.0.1/10.x/169.254.169.254, и нестандартные кодировки IP проходят и диалятся внутрь; тело кэшируется для doc read. Redirect-ревалидация — та же дыра.

## Acceptance Criteria

- проверка публичности адреса происходит в момент диала (DialContext/Control-хук) по резолвеному IP, а не только по строке host
- redirect на внутренний адрес блокируется тем же механизмом
- allowed_hosts в политике сохраняет семантику явного доверия
- тесты: loopback/приватный IP блокируется, публичный хост проходит
