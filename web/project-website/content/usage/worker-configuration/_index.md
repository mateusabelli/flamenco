---
title: Worker Configuration
weight: 3
---

Flamenco Worker uses two different configuration files. One can be shared
between all your Workers, if you so incline. The other should be strictly
separate for each Worker.

## Main Configuration File

Flamenco Worker will read its configuration from `flamenco-worker.yaml` in the
worker's *current working directory*. This file can be shared between all
Workers, if you want.

This is an example of such a configuration file:

```yaml
manager_url: http://flamenco.local:8080/
task_types: [blender, ffmpeg, file-management, misc]
restart_exit_code: 47

# Optional advanced option, available on Linux only:
oom_score_adjust: 500
```

- `manager_url`: The URL of the Manager to connect to. If the setting is blank
  (or removed altogether) the Worker will try to auto-detect the Manager on the
  network.
- `task_types`: The types of tasks this Worker is allowed to run. Task types are
  determined by the [job compiler scripts][scripts]; the ones listed here are in
  use by the default scripts. These determine which kind of tasks this Worker
  will get. See [task types][task-types] for more info.
- `restart_exit_code`: Having this set to a non-zero value will mark this Worker
  as 'restartable'. See [Shut Down & Restart Actions][restarting] for more
  information.
- `oom_score_adjust`: an optional value between 0 and 1000, only available on
  Linux. It configures the Out Of Memory behaviour of the Linux kernel. This is
  the `oom_score_adj` value for all sub-processes started by the Worker. Set
  this to a high value, so that when the machine runs out of memory when
  rendering, it is Blender that gets killed, and not Flamenco Worker itself. For
  more information, see [Linux Kernel: Per-Process Parameters][per-process-proc].
  *This option was introduced in Flamenco v3.5.*

[scripts]: {{< ref "usage/job-types" >}}
[task-types]: {{< ref "usage/job-types" >}}#task-types
[restarting]: {{< ref "usage/worker-actions" >}}#shut-down--restart-actions
[per-process-proc]: https://docs.kernel.org/filesystems/proc.html#chapter-3-per-process-parameters

## Worker-Specific Files

Apart from the above configuration file, which can be shared between Workers,
each Worker has a set of files that are specific to that Worker. These contain
the *worker credentials* (`flamenco-worker-credentials.yaml`), which are used to
identify this worker to the Manager, and a *database file*
(`flamenco-worker.sqlite`) to queue task updates when the Manager is
unreachable.

These files are stored in a platform-specific location:

| Platform | Default location                                              |
|----------|---------------------------------------------------------------|
| Linux    | `$HOME/.local/share/flamenco`                                 |
| Windows  | `C:\Users\UserName\AppData\Local\Blender Foundation\Flamenco` |
| macOS    | `$HOME/Library/Application Support/Flamenco`                  |

These files are not intended to be manually edited. If you want to reset your
Worker and make it act like it's brand new, shut down the worker, delete these
files, and restart the Worker again. Be sure to delete the old Worker from the
Flamenco Manager web interface as well.

## Configuration from Environment Variables

Certain settings can be configured via environment variables.

- `FLAMENCO_HOME`: Directory for [Worker local files](#worker-local-files). If
  not given, the above defaults are used.
- `FLAMENCO_WORKER_NAME`: The name of the Worker. If not specified, the Worker
  will use the hostname.
