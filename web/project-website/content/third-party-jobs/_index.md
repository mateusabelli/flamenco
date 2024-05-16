---
title: Third-Party Jobs
weight: 30
---

This section contains third-party job types for Flamenco. These have been
submitted by the community. If you wish to contribute your custom job type,
either

- join the [#flamenco Blender Chat channel][flamencochannel] and poke `@dr.sybren`, or
- write an [issue in the tracker][tracker] with your proposal.


## How can I create my own Job Type?

This is described [Job Types][jobtypes]. It is recommended to use the
[built-in scripts][built-in-scripts] as examples and adjust them from there.

## Installation

Each job type consists of a `.js` file. After downloading, place those in the
`scripts` directory next to the Flamenco Manager executable. Create the
directory if necessary. Then restart Flamenco Manager and in Blender press the
"Refresh from Manager" button.

## Third-Party Job Types

{{< flamenco/toc-children >}}

[jobtypes]: {{< ref "usage/job-types" >}}
[built-in-scripts]: https://projects.blender.org/studio/flamenco/src/branch/main/internal/manager/job_compilers/scripts
[flamencochannel]: https://blender.chat/channel/flamenco
[tracker]: https://projects.blender.org/studio/flamenco/issues/new?template=.gitea%2fissue_template%2fjobtype.yaml
