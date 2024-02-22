---
title: Failure Handling
draft: true
---

Flamenco has a few different approaches to try and complete a job, even when
workers sometimes fail at certain tasks. These are the thoughts behind its fault
handling design:

- When a worker fails a task, it **might be a problem with the worker** (maybe
  not enough memory to render this shot) or it may be a problem with the blend
  file it's rendering.
- **A worker only gets to fail at a task once**, so if it fails the task it
  won't get that one again, and the task is marked `soft-failed`. If you have
  multiple workers, then up to 3 workers (by default, see
  task_fail_after_softfail_count: 3` in the `flamenco-manager.yaml`) can have a
  go at the task before the task is really considered `failed`.
- When a worker fails multiple tasks of a job (by default also 3, see
  `blocklist_threshold: 3` in `flamenco-manager.yaml`) it gets blocklisted and
  is no longer allowed to do tasks of that type for that job.

There are two reasons a job may fail:

1. If there are too **many failed tasks**, the job will fail.
2. If there is **no worker left** to complete the job, the job will fail. Note
   that this also considers offline/asleep workers as "available", as they may
   come online at any moment. Or maybe that'll never happen, but that's not
   something Flamenco knows. Be sure to delete stale workers.
