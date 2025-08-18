---
title: Manager Configuration
weight: 3
---

Flamenco Manager reads its configuration from `flamenco-manager.yaml`, located
next to the `flamenco-manager` executable. The previous chapters
([Shared Storage][shared-storage] and [Variables][variables]) also describe parts of
that configuration file.

[shared-storage]: {{< ref "shared-storage" >}}
[variables]: {{< ref "usage/variables" >}}

## Example

```yaml
# flamenco-manager.yaml

_meta:
  version: 3

# Core settings
manager_name: Flamenco Manager
database: flamenco-manager.sqlite
listen: :8080
autodiscoverable: true

# Storage
local_manager_storage_path: ./flamenco-manager-storage
shared_storage_path: /path/to/storage
shaman:
  enabled: true
  garbageCollect:
    period: 24h
    maxAge: 744h

# Timeout & Failures
task_timeout: 10m
worker_timeout: 1m
blocklist_threshold: 3
task_fail_after_softfail_count: 3

# Variables
variables:
  blender:
    values:
      - platform: linux
        value: blender
      - platform: windows
        value: blender
      - platform: darwin
        value: blender
  blenderArgs:
    values:
      - platform: all
        value: -b -y

# MQTT Configuration
mqtt:
  client:
    broker: 'tcp://mqttserver.local:1883'
    username: 'username'
    password: 'your-password-here'
    topic_prefix: flamenco
```

The usual way to create a configuration file is simply by starting Flamenco
Manager. If there is no config file yet, it will start the setup assistant to
create one. If for any reasons the setup assistant is not usable for you, you
can use the above example to create `flamenco-manager.yaml` yourself.

## Definitions

The configuration is stored in a [YAML](https://spacelift.io/blog/yaml#basic-yaml-syntax) file. Each attribute is defined below.

### Duration Format

Durations are written in [Go's notation for durations][ParseDuration]. Examples
are `1h` for 1 hour, or `1m30s` for 1 minute and 30 seconds. To avoid ambiguity,
hours are the largest available unit; there are days that are not exactly `24h`.

[ParseDuration]: https://pkg.go.dev/time#ParseDuration

### Core Settings

---

`manager_name` string

The name of the Flamenco Manager.

---

`database` string

The file path for the SQLite database.

---

`listen` string

The IP and port (e.g., `:8080`, `192.168.0.1:8080`, or `[::]:8080`) Flamenco Manager will listen on.

This is the only port that is needed for Flamenco Manager, and will be used for the web interface, the API, and file submission via the Shaman system.

---

`autodiscoverable` boolean

Whether or not the manager is discoverable by workers in the same network.

### Storage

---

`local_manager_storage_path` string

The path where the Manager stores local files (e.g., logs, last-rendered images, etc.).

These files are only necessary for the manager. Workers never need to access this directly, as the files are accessible via the web interface.

---

`shared_storage_path` string

The [Shared Storage][shared-storage] path where files shared between Manager and Worker(s) live (e.g., rendered output files, or the _.blend_ files of render jobs).

---

`shaman` map

The configuration for enabling and garbage collecting the [Shaman Storage System][shaman].

[shaman]: {{< ref "usage/shared-storage/shaman.md" >}}

The exact structure for `shaman` follows:

```yaml
shaman:
  enabled: true
  garbageCollect:
    period: 24h
    maxAge: 744h
```

---

`enabled` boolean

Whether or not to use the Shaman Storage System.

---

`garbageCollect` map

The configuration for [garbage collection][garbage-collection] on files in the Shaman Storage System.

[garbage-collection]: {{< ref "usage/shared-storage/shaman.md#garbage-collection" >}}

---

`period` string in [duration format](#durations)

The period of time determining the frequency of garbage collection performed on file store.

---

`maxAge` string in [duration format](#durations)

The minimum lifespan of files required in order to be garbage collected.

### Timeout & Failures

---

`task_timeout` string in [duration format](#durations)

The Manager will consider a Worker to be "problematic" if it hasn't heard anything from that Worker for this amount of time. When that happens, the Worker will be shown on the Manager in `error` status.

---

`worker_timeout` string in [duration format](#durations)

The amount of time since the worker's last sign of life (e.g., asking for a task to perform, or checking if it's allowed to perform its current task) before getting marked "timed out" and sent to `error` status.

---

`blocklist_threshold` number

The number of failures allowed on a type of task per job before banning a worker from that task type on that job.

For example, when a worker fails multiple blender tasks on one job, it's concluded that the job is too heavy for its hardware, and thus it gets blocked from doing more of those. It is then still allowed to do file management, video encoding tasks, or blender tasks on another job.

---

`task_fail_after_softfail_count` number

The number of workers allowed to have failed a task before hard-failing the task.

### Variables

---

`variables` map

The [two-way variables][two-way-variables] to be used for specific operating systems.

[two-way-variables]: {{< ref "usage/variables/multi-platform" >}}

The structure for `variables` follows:

```yaml
variables:
  <variable-name>:
    values:
      - platform: linux
        value: value for Linux
      - platform: windows
        value: value for Windows
      - platform: darwin
        value: value for macOS
```

---

`<variable-name>` map

The variable (e.g., `blender`, `blenderArgs`, or `my_storage`) to be defined.

---

`values` array

The list of variable values with their respective platform to be used.

---

`platform` string

The platform for this variable value.

Possible values: `linux`, `windows`, `darwin`, or `all`.

Any other value used by Go's [`GOOS`](https://pkg.go.dev/runtime#GOOS) constant or returned by Python's [`platform.platform()`](https://docs.python.org/3/library/platform.html#platform.platform) function can be used here. Of the above values, only `all` is special as it pertains to all platforms.

---

`value` string

The contents for the variable, for the given platform.

---

For more information, see [Variables][variables].

### MQTT Configuration

This section is completely optional. If you do not know what it's for, just leave it out.

---

`mqtt` map

The configuration for MQTT broker and client.

The exact structure for `mqtt` follows:

```yaml
mqtt:
  client:
    broker: ''
    username: ''
    password: ''
    topic_prefix: ''
```

---

`client` map

The configuration for the broker and client.

---

`broker` string

The URL for the MQTT server.

---

`username` string

The username of the broker/client.

---

`password` string

The password of the broker/client.

---

`topic_prefix` string

The word to prefix each topic (e.g., `flamenco`).

---

For more information about the built-in MQTT client, see
[Manager Configuration: MQTT][mqtt].

[mqtt]: {{< ref "usage/manager-configuration/mqtt.md" >}}
