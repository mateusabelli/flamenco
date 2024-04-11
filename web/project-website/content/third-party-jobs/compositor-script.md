---
title: Compositor Nodes
weight: 10
---

*Job type documented and maintained by: [Dylan Blanqué][author]. Please report any issues at [the script's Github project][github].*

[author]: https://projects.blender.org/Dylan-Blanque
[github]: https://github.com/dblanque/flamenco-compositor-script/issues

{{< hint >}}

This is a community-made job type. It may not reflect the same design as the
rest of Flamenco, as it was made for a specific person to solve a specific need.

{{< /hint >}}


This job type updates Blender's compositor nodes to work with Flamenco.

You'll need to do the following changes to support this workflow:

1. Download the [Flamenco Compositor Script ZIP file][compositorrepo] and extract it somewhere.
2. Copy `startup_script.py` to the configured Blender File Folder in your shared storage.
3. Copy `multi_pass_render.js` to the `scripts` folder in your Flamenco Manager installation folder (create it if it doesn't exist).
4. Add these variables to your `flamenco-manager.yaml` file:
    - `storagePath`: Your NAS path, multi-platform variable.
    - `jobSubPath`: Where the jobs are stored inside `storagePath`.
    - `renderSubpath`: Where the render output is stored inside `storagePath`.
    - `deviceType`: Compute Device Type to force. Do not set the variable if you wish to use whatever is available.
5. Submit your job from Blender with the corresponding Multi-Pass Job, it should
whatever compositor nodes you have set and correct the paths where necessary.

[compositorrepo]: https://github.com/dblanque/flamenco-compositor-script/archive/refs/heads/main.zip

{{< hint type=warning >}}
This has only been tested in an environment with [Shaman][shaman] enabled, but it should work without Shaman as well.

[shaman]: {{< ref "/usage/shared-storage/shaman" >}}
{{< /hint >}}


# Example Configuration Flamenco Manager YAML

```yaml
# Configuration file for Flamenco.
#
# For an explanation of the fields,
# refer to the original flamenco-manager-example.yaml

_meta:
  version: 3
manager_name: Flamenco Manager
database: flamenco-manager.sqlite
listen: :8080
autodiscoverable: true
local_manager_storage_path: ./flamenco-manager-storage
shared_storage_path: /mnt/storage/project_files
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
    - platform: all
      value: blender
    - platform: linux
      value: /usr/local/blender/blender
    - platform: windows
        value: C:/Program Files/Blender Foundation/Blender 3.4/blender.exe
    - platform: darwin
        value: /usr/bin/blender
  blenderArgs:
    values:
    - platform: all
      value: -b -y
  storagePath:
    values:
    - platform: linux
      value: /mnt/storage
    - platform: windows
      value: "Z:\\"
  jobSubPath:
    values:
    - platform: all
      value: project_files
  renderSubPath:
    values:
    - platform: all
      value: project_render
  deviceType:
    values:
    - platform: all
      value: "CUDA"
    # Set the device type to FIRST or remove the variable definition
    # to use whatever device type is detected first.
```
