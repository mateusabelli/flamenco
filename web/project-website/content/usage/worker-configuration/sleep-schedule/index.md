---
title: Sleep Schedule

resources:
  - name: screenshot
    src: "flamenco-sleep-schedule-web-interface.png"
    title: Sleep Schedule of a Worker in the Flamenco Manager web interface
---

Workers can be given a sleep schedule. This tells the Worker when to go to
sleep, and when to wake up. A typical use is for a desktop computer that is **in
use during office hours**, and outside those hours be **part of the render
farm**.

{{< img name="screenshot" size="origin" >}}

The sleep schedule determines when Flamenco Worker is asleep, i.e. when it is
not active on the farm. You can also see this as a configuration of **when
someone else wants to use the computer**.

Status
: Whether the sleep schedule is doing anything. This can be toggled with the slider next to the "Sleep Schedule" header. If it's disabled, you can still edit it, but otherwise it is ignored. In this case the Worker can be woken up or sent to sleep manually via [worker actions]({{< ref "usage/worker-actions" >}}).

Days of the week
: Days of the week that this worker should be asleep. Write each day name using their first two letters, separated by spaces. For example: `mo tu we th fr`. Note that this does **not** support range notation (`mo-fr`).

Start Time & End Time:
: Start and end time of when this worker should be asleep, in 24h notation.

## Example

If the Worker machine is used by someone who works on weekdays except Wednesdays, usually from 10:00 to 19:00, the sleep schedule would look like this:

- **Days of the week:** `mo tu th fr`
- **Start Time:** `10:00`
- **End Time:** `19:00`
