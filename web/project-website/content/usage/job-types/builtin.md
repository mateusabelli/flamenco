---
title: Built-in Job Types
weight: 10
---

Flamenco comes with built-in job types that are used for most common tasks. Currently, there are two of them:

- Simple Blender Render
- Single Image Render

## Simple Blender Render

This built-in job type is used for rendering a sequence of frames from a single Blender file, and potentially creating a preview video for compatible formats using FFmpeg. This job type is suitable for straightforward rendering tasks where one needs to render a range of frames and potentially compile them into a video. Note that this job type does not render into video formats directly, so the output format should be FFmpeg-compatible image formats.

The job type defines several settings that can be configured when submitting a job:

- `Frames` _string, required_: The frame range to render, e.g. '47', '1-30', '3, 5-10, 47-327'. It could also be set to use scene range or automatically determined on submission.
- `Chunk Size` _integer, default: 1_: Number of frames to render in one Blender render task.
- `Render Output Root` _string, required_: Base directory where render output is stored. Job-specific parts will be appended to this path.
- `Add Path Components` _integer, required, default: 0_: Number of path components from the current blend file to use in the render output path.
- `Render Output Path` _non-editable_: Final file path where render output will be saved. This is a computed value based on the `Render Output Root` and `Add Path Components` settings.

By using this job type, you can easily distribute Blender rendering tasks across multiple workers in your Flamenco setup, potentially saving significant time on large rendering projects.

## Single Image Render

This built-in job type is designed for distributed rendering of a single image from a Blender file. It splits the image into tiles, renders each tile separately, and then merges the tiles back into a single image. This approach allows for parallel processing of different parts of the image, potentially speeding up the rendering process.

Currently, the job type supports composition, as long as there is one single `Render Layers` node. The job type does not support `Denoising` node.

The job type defines several settings that can be configured when submitting a job:

- `Tile Size X` _integer, default: 64: Tile size in pixels for the X axis, does not need to be divisible by the image width.
- `Tile Size Y` _integer, default: 64: Tile size in pixels for the Y axis, does not need to be divisible by the image height.
- `Frame` _integer, required_: The frame to render. By default, it uses the current frame in the Blender scene.
- `Render Output Root` _string, required_: Base directory where render output is stored. Job-specific parts will be appended to this path.
- `Add Path Components` _integer, required, default: 0_: Number of path components from the current blend file to use in the render output path.
- `Render Output Path` _non-editable_: Final file path where render output will be saved. This is a computed value based on the `Render Output Root` and `Add Path Components` settings.

Choosing the right tile size is crucial for performance. Too small tiles might increase overhead, while too large tiles might not distribute the workload effectively.
