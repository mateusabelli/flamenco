---
title: Cycles/OPTIX + Experimental
weight: 20

resources:
  - name: screenshot
    src: cycles-optix-gpu.png
    title: Screenshot of the Flamenco job submission panel in Blender
---

{{< flamenco/thirdPartyCompatibility blender="v4.2-alpha+" flamenco="v3.5+" >}}
Documented and maintained by [Sybren Stüvel][author].
Please report any issues at [Flamenco's tracker][tracker].

[author]: https://projects.blender.org/dr.sybren
[tracker]: https://projects.blender.org/studio/flamenco/issues
{{< /flamenco/thirdPartyCompatibility >}}

This job type is the most-used one at [Blender Studio](https://studio.blender.org/). It includes a few features:

- Always enable GPU rendering with OPTIX.
- Checkboxes to enable specific experimental flags.
- Extra input fields for arbitrary commandline arguments for Blender.

To use, download [cycles_optix_gpu.js](cycles_optix_gpu.js) and place it in the
`scripts` directory next to the Flamenco Manager executable. Create the
directory if necessary. Then restart Flamenco Manager and in Blender press the
"Refresh from Manager" button.

<style>
  figure {
    width: 30em;
  }
</style>

{{< img name="screenshot" size="medium" >}}
