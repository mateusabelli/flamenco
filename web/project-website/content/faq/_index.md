---
title: Frequently Asked Questions
weight: 10
geekdocNav: false
geekdocHidden: true
---
{{< toc format=html >}}

## Features

### What is new in Flamenco 3?

The more interesting changes in Flamenco versions are listed in the [changelog][changelog].

[changelog]: https://projects.blender.org/studio/flamenco/src/branch/main/CHANGELOG.md


### ​What's the difference with OpenCue?

OpenCue is aimed at a different audience than Flamenco. OpenCue is a large and
complex project, and relies on a lot of components
([source](https://www.opencue.io/docs/getting-started/)), whereas Flamenco is
made for simplicity and use in small studios or at home, running on your own
hardware.


### I use a mix of different operating systems, can I still use Flamenco?

Yes, absolutely. To support multiple platforms, first configure your Manager for
its own platform (so if you run that on Linux, use Linux paths). Then you can
use [Two-way Variables][twovars] to translate those paths to the other
platforms.

Do note that Flamenco was developed on Linux, for the Linux-only [Blender
Studio][studio]. You may find issues that the developers did not run into
themselves. If you do, please [report a bug][bug].

[twovars]: {{< ref "usage/variables/multi-platform" >}}
[studio]: https://studio.blender.org/
[bug]: https://projects.blender.org/studio/flamenco/issues/new?template=.gitea%2fissue_template%2fbug.yaml


### How do I make the Workers render on GPU?

Blender needs to know which method to use (CUDA, Optix, etc.), and on which
video card. This is something you'll need to configure yourself. Start Blender
on the worker machine, and then [update the preferences][cycles-gpu-prefs]. If
you disabled automatic saving of preferences, be sure to save them before you
quit Blender.

Also be sure to *not* use `--factory-startup` in your Blender CLI arguments, as
that will reset Blender back to using the CPU. If you're using bog-standard
Flamenco settings, then don't worry about this, it doesn't use that option.

[cycles-gpu-prefs]: https://docs.blender.org/manual/en/latest/editors/preferences/system.html#cycles-render-device


### Can I make a Worker render on a specific GPU?

In short: not really, not only with Flamenco, anyway.

The issue is that Blender has no commandline options to reliably select the
GPU(s) to use. The last information the Flamenco team heard of this was that
certain drivers are unreliably reporting the GPU order when they are of
identical make & model. So you could in theory tell Blender to run on GPU #1,
and another Blender on GPU #2, but since the second Blender may see them in a
different order, in the end both would use the same GPU, and one GPU would be
idle.

A possible workarounds could be to have multiple copies of Blender on your
computer, and using the utilities of your video card to configure each Blender
copy to only use a specific GPU. Then run multiple Flamenco Workers on that
machine, each having a different Blender on its `$PATH`.


### Can I change the paths/names of the rendered files?

Where Flamenco places the rendered files is determined by the job type. You can
create [your own custom job type][jobtypes] or check the existing
[third-party job types][thirdpartyjobs] to change this. With that, you can
even add your own custom job settings like a sequence identifier and use that to
determine the location of rendered files.


### Can Flamenco render a single image across multiple Workers?

Flamenco does not support this at the moment. In theory this would be possible
with a [custom job type][jobtypes]. With the Cycles render engine it might be
possible it set up a set of tasks that each render a specific chunk of samples,
and then merge those samples together for the final image.

If you have made a custom job type that does this, please contact us to get it
added to the [third-party jobs section][thirdpartyjobs].


### Can I use the Compositor to output multiple EXR files or Passes?

This is possible with Flamenco, but it takes a bit of work. Although it's not
managed by Flamenco's default job types, you can use a [custom job type][jobtypes]
for this.

With that, you have control over the arguments that get used before and/or after
the filename on the CLI.

There are Flamenco jobs out there that support compositor nodes,
multi-platform, and multiple pass outputs. You can check our [third-party jobs
section][thirdpartyjobs].

If you wish to contribute to the project, you're invited to
[get involved with Flamenco][getinvolved]!

[jobtypes]: {{< ref "usage/job-types" >}}
[thirdpartyjobs]: {{< ref "third-party-jobs" >}}
[getinvolved]: {{< ref "development/get-involved" >}}


### Can I use SyncThing, Dropbox, Google Drive, or other file syncing software?

In short: No. Sharing files using Syncthing, OwnCloud, Dropbox, Google Drive,
Onedrive, etc. is not supported by Flamenco. For more information, see [Cloud
Storage Services][cloud-storage].

[cloud-storage]: {{< ref "/usage/shared-storage" >}}#cloud-storage-services


## Troubleshooting

### My Worker cannot find my Manager, what do I do?

First check the Manager output on the terminal, to see if it shows any messages
about "auto-discovery" or "UPnP/SSDP". Most of the time it's actually Spotify
getting in the way, so make sure to close that before you start the Manager.

If that doesn't help, you'll have to tell the Worker where it can find the
Manager. This can be done on the commandline, by running it like
`flamenco-worker -manager http://192.168.0.1:8080/` (adjust the address to your
situation) or more permanently by [editing the worker configuration
file][workercfg].

[workercfg]: {{< ref "usage/worker-configuration" >}}

### My Worker cannot find Blender, what do I do?

When installing and starting the Flamenco Worker you may see a warning in the logs that says
the Worker cannot find Blender.

```
WRN Blender could not be found. Flamenco Manager will have to supply the full path to Blender when Tasks are sent to this Worker. For more help see https://flamenco.blender.org/usage/variables/blender/
```

If Flamenco cannot locate Blender on the system it is possible to use a [two-way variable named `blender`][blendervar] for each platform (eg: Windows, Linux, or MacOS). This path to Blender is then sent to the Worker for each render task. Note that the Worker will still show the warning at startup, as it cannot find Blender by itself; this is fine, because you now have configured the Manager to provide this path.

[blendervar]: {{< ref "usage/variables/blender" >}}

### What do "Error: Cached job type is old" or "job type etag does not match" mean?

This means that you have to click on the little "Refresh" icon next to the job type:

<img src="job-types-refresh.webp" width="396" height="41">


### Render jobs hang after the first chunk of frames, what's wrong?

When rendering a chunk of frames, Flamenco waits until Blender quits. This
signals Flamenco that it finished rendering. Sometimes an add-on prevents
Blender from quitting, and thus Flamenco will think it is still doing something.
Disable add-ons one-by-one to see which one is causing this issue.


### What does "command exited abnormally with code 1" mean?

It means that the program (probably Blender) exited with an error status. Take a
look at the task log, which you can access by going to the task in Flamenco's
web interface.
