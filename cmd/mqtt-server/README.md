# MQTT Server

This is a little MQTT server for test purposes. It logs all messages that
clients publish.

**WARNING:** This is just for test purposes. There is no encryption, no
authentication, and no promise of any performance. Havnig said that, it can be
quite useful to see all the events that Flamenco Manager is sending out.

## Running the Server

```
go run ./cmd/mqtt-server
```

## Connecting Flamenco Manager

You can configure Flamenco Manager for it, by setting this in your
`flamenco-manager.yaml`:

```yaml
mqtt:
  client:
    broker: "tcp://localhost:1883"
    clientID: flamenco
    topic_prefix: flamenco
    username: ""
    password: ""
```
