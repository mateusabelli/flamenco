---
title: Manager Configuration
weight: 3
---

Flamenco Manager reads its configuration from `flamenco-manager.yaml`, located
next to the `flamenco-manager` executable. The previous chapters
([Shared Storage][storage] and [Variables][variables]) also described parts of
that configuration file.

[storage]: {{< ref "shared-storage" >}}
[variables]: {{< ref "usage/variables/multi-platform" >}}

## Example Configuration

This is an example `flamenco-manager.yaml` file:

```yaml
_meta:
  version: 3

manager_name: Flamenco Manager
database: flamenco-manager.sqlite
listen: :8080
autodiscoverable: true

local_manager_storage_path: ./flamenco-manager-storage
shared_storage_path: /path/to/storage
shaman:
  enabled: true
  garbageCollect:
    period: 24h0m0s
    maxAge: 744h0m0s
    extraCheckoutPaths: []

task_timeout: 10m0s
worker_timeout: 1m0s
blocklist_threshold: 3
task_fail_after_softfail_count: 3

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

mqtt:  # See 'MQTT Configuration' below.
  client:
    broker: "tcp://mqttserver.local:1883"
    username: "username"
    password: "your-password-here"
    topic_prefix: flamenco

```

The usual way to create a configuration file is simply by starting Flamenco
Manager. If there is no config file yet, it will start the setup assistant to
create one. If for any reasons the setup assistant is not usable for you, you
can use the above example to create `flamenco-manager.yaml` yourself.

## MQTT Configuration

The `mqtt` section is completely optional. If you do not know what it's for, just leave it out.

For more information about the built-in MQTT client, see
[Manager Configuration: MQTT][mqtt].

[mqtt]: {{< ref "usage/manager-configuration/mqtt.md" >}}
