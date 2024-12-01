---
title: Single Image
weight: 5
---

{{< flamenco/thirdPartyCompatibility blender="v4.2+" flamenco="v3.6-alpha+" >}}

Created by [David Zhang][author].
Documented and maintained by [Sybren Stüvel][maintainer].
Please report any issues at [Flamenco's tracker][tracker].

[author]: https://projects.blender.org/David-Zhang-10
[maintainer]: https://projects.blender.org/dr.sybren
[tracker]: https://projects.blender.org/studio/flamenco/issues
{{< /flamenco/thirdPartyCompatibility >}}

This job type can render an image by splitting it up into tiles and assigning
those tiles to different workers. As the last task in the job, those tiles are
merged into the final output image.

To use, download [single_image_render.js](single_image_render.js) and place it
in the `scripts` directory next to the Flamenco Manager executable. Create the
directory if necessary. Then restart Flamenco Manager and in Blender press the
"Refresh from Manager" button.

## Limitations

There are a few limitations of this script:

- Only supports 100% render scale.
- Does not support denoising, as Blender doesn't expose enough info in a way
  that can be exported to tiled multi-layer EXR.
- Needs more testing before it can be bundled with Flamenco itself.

For more information, please see [GSoC 2024: Improve Distributed Rendering & Task Execution][devtalk] on devtalk.

[devtalk]: https://devtalk.blender.org/t/gsoc-2024-improve-distributed-rendering-task-execution/34566/14
