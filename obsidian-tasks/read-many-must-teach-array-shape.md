---
id: read-many-must-teach-array-shape
title: read/read_many must teach the paths array shape
status: done
priority: high
task_type: bug
tags:
    - mcp
    - schema
    - teaching-errors
    - fileops
created_at: "2026-08-20T17:36:35.040584Z"
updated_at: "2026-08-20T17:40:26.061786Z"
---

## Body

Модель звала file action=read_many без массива paths, получала невнятную ошибку и уходила в shell/sed. Три ловушки: (1) bind молча глотает paths в action=read → «requires path»; (2) read_many без paths не показывает форму массива; (3) singular path вместо paths не подхватывается. чинится teaching-ошибками в духе v1.2.1.
