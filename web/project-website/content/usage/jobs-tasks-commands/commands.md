---
title: List of Commands
weight: 6
---

The following commands are implemented in Flamenco Worker.

{{< toc >}}


## Blender: `blender-render`

Runs Blender. Command parameters:

| Parameter    | Type       | Description                                                                                                                                                                                                                                            |
|--------------|------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `exe`        | `string`   | Path to a Blender executable. Typically the expansion of the `{blender}` [variable][variables]. If set to `"blender"`, the Worker performs a search on `$PATH` and on Windows will use the file association for the `.blend` extension to find Blender |
| `exeArgs`    | `string`   | CLI arguments to use before any other argument. Typically the expansion of the `{blenderargs}` [variable][variables]                                                                                                                                   |
| `argsBefore` | `[]string` | Additional CLI arguments defined by the job compiler script, to go before the blend file.                                                                                                                                                              |
| `blendfile`  | `string`   | Path of the blend file to open.                                                                                                                                                                                                                        |
| `args`       | `[]string` | Additional CLI arguments defined by the job compiler script, to go after the blend file name.                                                                                                                                                          |

[variables]: {{< ref "/usage/variables" >}}

The constructed CLI invocation will be `{exe} {exeArgs} {argsBefore} {blendfile} {args}`.

Flamenco Worker monitors the logging of Blender; lines like `Saved: filename.jpg` are recognised and sent as preview images to Flamenco Manager.

## FFmpeg: `frames-to-video`

Uses FFmpeg to convert an image sequence to a video file.

| Parameter    | Type       | Description                                                                                                                             |
|--------------|------------|-----------------------------------------------------------------------------------------------------------------------------------------|
| `exe`        | `string`   | Path to the `ffmpeg` executable. If it is set to `"ffmpeg"` the Worker will look in its `tools` directory first, and search on `$PATH`. |
| `exeArgs`    | `string`   | Its CLI parameters defined by the Manager.                                                                                              |
| `fps`        | `float64`  | Frames per second of the video file.                                                                                                    |
| `inputGlob`  | `string`   | Glob of input files.                                                                                                                    |
| `outputFile` | `string`   | File to save the video to.                                                                                                              |
| `argsBefore` | `[]string` | CLI arguments to use before the                                                                                                         |
| `args`       | `[]string` | Additional CLI arguments defined by the job compiler script, to between the input and output filenames.                                 |

The constructed CLI invocation will be `{exe} {exeArgs} {argsBefore} {platform-dependent inputGlob} {args} -r {fps} {outputFile}`, where `{platform-dependent inputGlob}` is determined by the OS of the executing Flamenco Worker.

## File Management: `move-directory`

Moves a directory from one path to another.

| Parameter | Type     | Description                    |
|-----------|----------|--------------------------------|
| `src`     | `string` | Path of the directory to move. |
| `dest`    | `string` | Destination to move it to.     |

If the destination directory already exists, it is first moved aside to a timestamped path  `{dest}-{YYYY-MM-DD_HHMMSS}` to its name. The timestamp is the 'last modified' timestamp of that existing directory.

## File Management: `copy-file`

Copies a file from one location to another.

| Parameter | Type     | Description                                                                       |
|-----------|----------|-----------------------------------------------------------------------------------|
| `src`     | `string` | Path of the file to copy. Must be an absolute path.                               |
| `dest`    | `string` | Destination to copy it to. Must be an absolute path. This path may not yet exist. |

## Misc: `exec`

Run an executable. This can be any executable. The command succeeds when the
executable exit status is `0`, and fails otherwise. The executable needs to stop
running (or fork) in order for the Worker to consider the command 'done'.

The executable is run directly, and *not* via a shell invocation. To run a shell
command, use something like `{exe: "/bin/bash", args: ["-c", "echo", "hello
world"]}`.


{{< hint type=info >}}
If there is a specific command for the functionality you need, like
`blender-render` or `ffmpeg`, use those commands instead. They are aware of
cross-platform differences, and know more about the program they are running.
For example, the `blender-render` command sends rendered images to the Manager
to show in the web interface, and `ffmpeg` will change its commandline arguments
depending on the platform it runs on.
{{< /hint >}}

| Parameter | Type       | Description            |
|-----------|------------|------------------------|
| `exe`     | `string`   | The executable to run. |
| `args`    | `[]string` | Commandline arguments. |

## Misc: `echo`

Writes a message to the task log.

| Parameter | Type     | Description         |
|-----------|----------|---------------------|
| `message` | `string` | The message to log. |

## Misc: `sleep`

Does nothing for a period of time.


| Parameter             | Type             | Description                                                        |
|-----------------------|------------------|--------------------------------------------------------------------|
| `duration_in_seconds` | `float` or `int` | The amount of time to sleep for, in seconds. Must be non-negative. |
