---
title: Shared Storage
---

Flamenco needs some form of *shared storage*: a place for files to be stored
that can be accessed by all the computers in the farm.

Basically there are three approaches to this:

| Approach                            | Simple | Efficient | Render jobs are isolated |
|-------------------------------------|--------|-----------|--------------------------|
| Work directly on the shared storage | ✅      | ✅         | ❌                        |
| Create a copy for each render job   | ✅      | ❌         | ✅                        |
| Shaman Storage System               | ❌      | ✅         | ✅                        |

Each is explained below.

{{< hint type=Warning >}}
On Windows, Flamenco **only supports drive letters** to indicate locations.
Flamenco does **not** support UNC notation like `\\SERVER\share`. Mount the
network share to a drive letter. The examples below use `S:` for this.
{{< /hint >}}

## Work Directly on the Shared Storage

Working directly in the shared storage is the simplest way to work with
Flamenco. You can enable this mode by pointing Flamenco at the location of your
blend files.

As an example, if `S:\WorkArea` is where your blend files live (or in a
subdirectory thereof), you can update your `flamenco-manager.yaml` like this:

```yaml
shared_storage_path: S:\WorkArea
shaman:
  enabled: false
```

When you submit a file from the shared storage, say
`S:\WorkArea\project\scene\shot\anim.blend`, Flamenco will detect this and
assume the Workers can reach the file there. No copy will be made.

This "work on shared storage" approach has the downside that render jobs are not
fully separated from each other. For example, when you change a texture while a
render job is running, the subsequently rendered frames will be using that
altered texture. If this is an issue for you, keep reading.

## Creating a Copy for Each Render Job

This approach will create a directory for the job, on the shared storage. It
will copy the submitted blend file, and all its dependencies, to that directory.
Because of this, each render job has its own set of files, and is independent
from other render jobs.

As an example, when `C:\WorkArea` is where you work on your blend files, and
`S:\Flamenco` is the shared storage for Flamenco, you will automatically use
this approach. You can update your `flamenco-manager.yaml` like this:

```yaml
shared_storage_path: S:\Flamenco
shaman:
  enabled: false
```

As you can see, you do not have to tell Flamenco about `C:\WorkArea`, it'll
automatically detect which storage approach to use from the path of the blend
file you're submitting.

The downside of this approach is that each render job has a completely
independent set of files. This means that file submission can be slow, because
for each render job all its dependencies will be copied. This can be avoided
with the Shaman system, explained below.

## Shaman Storage System

The Shaman system ensures that files are only copied once to the render farm.

This is explained in a page of its own; see [Shaman Storage System][shaman].

[shaman]: {{< relref "shaman" >}}


## Cloud Storage Services

Sharing files using Syncthing, OwnCloud, Dropbox, Google Drive, Onedrive, etc.
is **not supported by Flamenco**.

Flamenco assumes that once a file has been written by one worker, it is
immediately available to any other worker, like what you'd get with a NAS.
Similarly, it assumes that when a job has been submitted, it can be worked on
immediately.

Such assumptions no longer hold true when using an asynchronous service like
SyncThing, Dropbox, etc.

Note that this is not just about the initially submitted files. Flamenco creates
a video from the rendered images; this also assumes that those images are
accessible after they've been rendered and saved to the storage.

It might be possible to create a complex [custom job type][jobtypes] for this,
but that's all untested. The hardest part is to know when all necessary files
have arrived on a specific worker, without waiting for *all* syncing to be
completed (as someone may have just submitted another job).

[jobtypes]: {{< ref "/usage/job-types" >}}

## Absolute vs. Relative Paths

Blender can reference assets (textures, linked blend files, etc.) in two ways:

- by **relative path**, like `//textures\my-favourite-brick.exr`, which is relative to the blend file, or
- by **absolute path**, like `D:\texture-library\my-favourite-brick.exr`, which is the full path of the file.

When an asset is referenced by an absolute path, **Flamenco assumes that this
path is valid for all Workers, and will not copy those assets to the shared
storage.** This makes it possible to store large files, like simulation caches,
on the shared storage, without Flamenco creating a copy for each render job.

{{< hint type=Warning >}} On Windows it is not possible to construct a relative
path to an asset when that asset is on a different drive than the main blend
file. If you still want Flamenco to copy such assets, there are two workarounds:

- Move your asset libraries to the same drive as your Blender projects.
- Use [symbolic links][symlinks-guide-windows] to make your assets available at
  a suitable path.

[symlinks-guide-windows]: https://www.howtogeek.com/16226/complete-guide-to-symbolic-links-symlinks-on-windows-or-linux/
{{< /hint >}}
