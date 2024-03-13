---
title: "Manager Configuration: MQTT"
titleTOC: MQTT
---

Flamenco Manager can send its internal events to an [MQTT][mqtt] broker. Other
MQTT clients can listen to those events, in order to respond to what happens on
the render farm.

[mqtt]: https://en.wikipedia.org/wiki/MQTT

*MQTT support was introduced in Flamenco 3.5.*

## Configuration

To enable MQTT functionality, place a section like this in your
`flamenco-manager.yaml` file and restart Flamenco Manager:

```yaml
mqtt:
  client:
    broker: "tcp://mqttserver.local:1883"
    username: "username"
    password: "your-password-here"
    topic_prefix: flamenco
```

<div>
<style>
  .gdoc-markdown dl dt {
    margin-top: 0.1rem;
  }
</style>

`broker`
: The URL of the MQTT Broker. Supports `tcp://` and `ws://` URLs.

`username` & `password`
: The credentials used to connect to the MQTT Broker. For anonymous access, just
  remove those two keys.

`topic_prefix`
: Topic prefix for the MQTT events sent to the broker. Defaults to `flamenco`.
  For example, job updates are sent to the `flamenco/jobs` topic.

</div>

## MQTT Topics

The following topics will be used by Flamenco. The `flamenco` prefix for the topics is configurable.

| Description                      | MQTT topic                               | JSON event payload        |
|----------------------------------|------------------------------------------|---------------------------|
| Manager startup/shutdown         | `flamenco/lifecycle`                     | `EventLifeCycle`          |
| Farm status                      | `flamenco/status`                        | `EventFarmStatus`         |
| Job update                       | `flamenco/jobs`                          | `EventJobUpdate`          |
| Task update                      | `flamenco/jobs/{job UUID}`               | `EventTaskUpdate`         |
| Worker update                    | `flamenco/workers`                       | `EventWorkerUpdate`       |
| Worker Tag update                | `flamenco/workertags`                    | `EventWorkerTagUpdate`    |
| Last rendered image              | `flamenco/last-rendered`                 | `EventLastRenderedUpdate` |
| Job-specific last rendered image | `flamenco/jobs/{job UUID}/last-rendered` | `EventLastRenderedUpdate` |

For the specification of the JSON sent in the MQTT events, use the above table
and then look up the type description in the [OpenAPI specification][oapi].

[oapi]: https://projects.blender.org/studio/flamenco/src/branch/main/pkg/api/flamenco-openapi.yaml
