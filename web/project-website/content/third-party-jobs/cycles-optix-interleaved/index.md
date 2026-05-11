---
title: Cycles/OPTIX Interleaved Rendering
weight: 25
---

{{< flamenco/thirdPartyCompatibility blender="v5.0+" flamenco="v3.8+" >}}
Documented and maintained by [Sybren Stüvel][author].
Please report any issues at [Flamenco's tracker][tracker].

[author]: https://projects.blender.org/dr.sybren
[tracker]: https://projects.blender.org/studio/flamenco/issues
{{< /flamenco/thirdPartyCompatibility >}}

This job type is used by [Blender Studio](https://studio.blender.org/welcome/). As usual, it splits
up the job into tasks that each render a number of frames.

Tasks are not handled sequentially, but in a different order:

- First & last tasks first.
- Then the middle one.
- Then the ones between the middle and the first/last ones.
- Then the ones between those.
- and it keeps subdividing until a limit is hit, and then all tasks are handled normally again.

Furthermore, it does this **interleaved with other jobs of the same priority**. So it renders a few
frames of one job, then a few frames of the next job. This makes it possible to inspect those frames
for missing textures and other issues.


To use, download [cycles_optix_gpu_interleaved.js](cycles_optix_gpu_interleaved.js) and place it in
the `scripts` directory next to the Flamenco Manager executable. Create the directory if necessary.
Then restart Flamenco Manager and in Blender press the "Refresh from Manager" button.
