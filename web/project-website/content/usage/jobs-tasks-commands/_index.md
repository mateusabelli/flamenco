---
title: Jobs, Tasks, and Commands
weight: 5
---

TODO: write about the pipeline from job submission to command execution.

## Job Statuses

The following table shows the meaning of the different job statuses:

| Status                    | Meaning | Possible next status |
| ------------------------- | ------- | ----------- |
| `under-construction`      | Preparing job for execution | `queued`, `active` |
| `queued`                  | Ready to be assigned to available Workers | `active`, `canceled` |
| `active`                  | Tasks assigned to Workers for execution | `completed`,  `canceled`, `failed` |
| `completed`               | All tasks executed successfully | `queued` |
| `failed`                  | Execution of one or more tasks failed after multiple retries by different Workers | `queued`, `canceled` |
| `cancel-requested`        | Request for job cancellation raised by user | `canceled` |
| `canceled`                | Canceled by the user, job terminated immediately on all Workers | `queued` |
| `requeueing`              | Request for requeueing of job raised by user | `queued` |
| `paused`                  | Not yet implemented | |

NOTE : 
- Requeueing a job when it is `completed`, i.e. when all its tasks are `completed`, will requeue all its tasks.
- Requeueing a job when some of its tasks are not `completed` will requeue these tasks only. The `completed` tasks will not be requeued, and will remain at `completed` status.

## Task Statuses

The following table shows the meaning of the different task statuses:

| Status        | Meaning | Possible next status |
| ------------- | ------- | ----------- |
| `queued`      | Ready to be assigned to an available Worker | `active`, `canceled` |
| `active`      | Assigned to a Worker for execution | `completed`, `canceled`, `failed`, `soft-failed` |
| `completed`   | Task executed successfully | `queued` |
| `soft-failed` | Same as `queued`, but has been failed by a Worker in an earlier execution | `queued`, `completed`, `failed`, `canceled` |
| `failed`      | Execution failed after multiple retries by different Workers | `queued`, `canceled` |
| `canceled`    | Canceled by the user, task terminated immediately | `queued` |
| `paused`      | Not yet implemented | |
