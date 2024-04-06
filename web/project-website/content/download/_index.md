---
title: Download
weight: 2
geekdocNav: false
geekdocHidden: true
---

Download Flamenco for your platform here. Each download contains both Flamenco
Manager and Worker. The Blender add-on can be downloaded from the Flamenco
Manager web-interface after installation.

The latest version is: **{{< flamenco/latestVersion >}}**

What is new is described in the {{< flamenco/releaseChangelogLink "release changelog" ->}}.

| Platform                           | File                                                                                         |
|------------------------------------|----------------------------------------------------------------------------------------------|
| Windows                            | {{< flamenco/downloadLink os="windows" ext="zip" >}}                                         |
| Linux                              | {{< flamenco/downloadLink os="linux" >}}                                                     |
| macOS <small>(Intel)</small>       | {{< flamenco/downloadLink os="macos" >}}                                                     |
| macOS <small>(Silicon/ARM)</small> | {{< flamenco/downloadLink os="macos" arch="arm64" >}} (without FFmpeg, [see below][mac-arm]) |
| checksums                          | {{< flamenco/sha256link >}}                                                                  |

Please report any issue at [projects.blender.org][bugs].

## Go Experimental!

The latest experimental version is: **{{< flamenco/latestExperimentalVersion
>}}**. This version is not guaranteed to be stable, so do not run this on
production systems. Or at least make a backup of your `flamenco-manager.yaml`
and `flamenco-manager.sqlite` files before you venture forth.

To see what's new, check [the development changelog](https://projects.blender.org/studio/flamenco/src/branch/main/CHANGELOG.md).

| Platform                           | File                                                                                                     |
|------------------------------------|----------------------------------------------------------------------------------------------------------|
| Windows                            | {{< flamenco/downloadExperimentalLink os="windows" ext="zip" >}}                                         |
| Linux                              | {{< flamenco/downloadExperimentalLink os="linux" >}}                                                     |
| macOS <small>(Intel)</small>       | {{< flamenco/downloadExperimentalLink os="macos" >}}                                                     |
| macOS <small>(Silicon/ARM)</small> | {{< flamenco/downloadExperimentalLink os="macos" arch="arm64" >}} (without FFmpeg, [see below][mac-arm]) |
| checksums                          | {{< flamenco/sha256linkExperimental >}}                                                                  |

Please report any issue at [projects.blender.org][bugs].

[bugs]: https://projects.blender.org/studio/flamenco/issues/new?template=.gitea%2fissue_template%2fbug.yaml
[mac-arm]: #macos-silicon-builds

<!--

{{< hint type=caution >}}
When **upgrading** from a previous experimental version, it is recommended to
start afresh with the following steps:

1. Cancel any running or queued job.
2. Shut down Flamenco Manager and all Workers.
3. Remove `flamenco-manager.yaml` and `flamenco-manager.sqlite`.
4. Download the new version and replace your old Flamenco files with the new ones.
5. Start `flamenco-manager` and go through the setup setup assistant again.
6. Don't forget to re-download the Blender add-on from the Manager's web
   interface, and install it. It has seen development as well, and will need to
   be upgraded.

[blog]: https://studio.blender.org/blog/announcing-flamenco-3-beta/
{{< /hint >}}
-->

## macOS "Silicon" builds

The macOS "Silicon" build does not ship with FFmpeg, because a trusted build for
this architecture is not provided by the FFmpeg project. This is why Flamenco v3
did not ship macOS/ARM64 builds. As of v3.3 this architecture is included in the
official Flamenco builds, but still without FFmpeg binary.

You can install FFmpeg using [the ffmpeg Homebrew formula][brew] or any other
method. Once installed Flamenco Worker should find it automatically. If not,
place the ffmpeg executable into Flamenco's `tools` directory.

[brew]: https://formulae.brew.sh/formula/ffmpeg


## License

Flamenco is Free and Open Source software, available under the
[GNU General Public License](https://projects.blender.org/studio/flamenco/src/branch/main/LICENSE).<br>
Download the source code at [projects.blender.org](https://projects.blender.org/studio/flamenco).
