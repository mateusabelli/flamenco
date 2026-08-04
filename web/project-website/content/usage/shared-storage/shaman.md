---
title: Shaman Storage System
---

{{< toc >}}

Flamenco comes with a storage system named _Shaman_. It makes it possible to
have independence of render jobs, as well as as-fast-as-possible uploads to the
farm. Shaman is built into Flamenco Manager.

- **As fast as possible:** only those files that have been newly created or
  modified need to be sent to the render farm. Files that have been uploaded
  before are automatically skipped.
- **Independence of render jobs:** each render job uses the files as they were
  at the moment the job was submitted. Subsequent modifications to those files
  will not influence that render job.

## How does it work?

When a render job is submitted from Blender using the Shaman system, the add-on
communicates with Flamenco Manager. Together they determine which files are
already available on the shared storage, and which still need uploading. Once
that's done, Shaman will recreate the file layout required for the render job.

When the Shaman system is enabled, Flamenco Manager creates two directories in
the shared storage:

- `file-store`: all the uploaded files are stored here. They are not stored by
  their original filename, but rather by an identifier that is based on their
  contents. In other words, when a file is renamed but otherwise is unchanged,
  it will still be identified as the same file.
- `jobs`: each render job will get its own directory here. It will contain
  _symbolic links_ (also known as _symlinks_) to the files in `file-store`. This
  way a file that was uploaded once can appear in multiple jobs simultaneously.

The process of submitting files via Shaman works as follows:

1. The Flamenco Blender add-on determines which files are necessary to render the current blend file.
2. It creates an _identifier_ for this file, which consists of the SHA256 sum + the length of the file in bytes.
3. A list of all identifiers is sent to Flamenco Manager.
4. Flamenco Manager checks which of the identified files are already available in the shared storage, and which ones should be uploaded.
5. The Blender add-on uploads these files.
6. The Blender add-on sends the list of identifiers again, this time together with the desired file path. For example, it will send entries like `8c6c3a96efed9637dfe2ed4966b7b0b42ebf291c3ae23895b53ed1da51c468ff 512 path/to/file.blend`.
7. Flamenco Manager creates a _checkout_ of the identified files, by creating the directory structure and using symbolic links to make the files available at the expected paths.

## Why is it called Shaman?

It was named this way because it uses SHA256 sums to identify files. Also it's a
[Sintel][sintel] reference, where one of the main characters is called _the shaman_.

[sintel]: https://studio.blender.org/films/sintel/

## Requirements

Because of the use of _symbolic links_ (also known as _symlinks_), using Shaman
is only possible on systems that support those. These should be supported by the
computers running Flamenco Manager and Workers.

### Windows

Shaman is not universally supported on Windows, as symlink behaviour on shared
network drives is complex.

The Shaman storage system uses _symbolic links_. On Windows the creation of
symbolic links requires a change in security policy. This can be done as
follows:

{{< tabs "shaman-windows" >}}
{{< tab "Windows Home / Core" >}}

On Windows Home (also known as "core"), you'll need to enable Developer Mode:

1. Press the Windows key, type "_Developer settings_", and click Open or press
   Enter.
2. Click the slider under "_Developer Mode_" to turn it ON.

See [Developer Mode][devmode] for more information, including some security implications.

Alternatively you can use the freely available [Polsedit][polsedit] to enable
the _Create Symbolic Links_ security policy.

[devmode]: https://learn.microsoft.com/en-us/windows/apps/get-started/enable-your-device-for-development
[polsedit]: https://www.southsoftware.com/polsedit.html

{{< /tab >}}
{{< tab "Windows Pro / Enterprise" >}}

On Windows Pro & Enterprise you need to enable a security policy.

1. Press Win+R, in the popup type `secpol.msc`. Then click OK.
2. In the _Local Security Policy_ window that opens, go to _Security Settings_ > _Local Policies_ > _User Rights Assignment_.
3. In the list, find the _Create Symbolic Links_ item.
4. Double-click the item and add yourself (or the user running Flamenco Manager or the whole users group) to the list.
5. Log out & back in again, or reboot the machine.

For more info see [the Microsoft documentation][secpol].

[secpol]: https://learn.microsoft.com/en-us/windows/security/threat-protection/security-policy-settings/create-symbolic-links

{{< /tab >}}
{{< /tabs >}}

#### Following Shaman symlinks over a shared network drive

By default, Windows **does not evaluate remote-to-remote (R2R) symbolic
links** (i.e. reparse points read over a shared network drive, also known as
SMB). Without that evaluation, Workers can see paths under `jobs/` that look
like files but fail to open; Blender will report
`ERROR File format is not supported`.

If Flamenco Manager runs on Windows and has the shared storage on a network
share, run this command in a Command Prompt (run as Administrator) on every
Windows worker:

```
fsutil behavior set SymlinkEvaluation R2R:1
```

A Worker that also hosts the share locally may not need this for its own
local access, but enabling R2R on all Windows Workers is the safer default.
Behaviour can differ by Windows edition; this has only been verified on
Windows 11 Pro.

This is a machine-wide Windows setting and a security trade-off: it relaxes a
symlink-attack mitigation. See the
[Create Symbolic Links][symlink-policy] policy documentation for background,
[symbolic link evaluation][symlink-eval] for the `fsutil` controls, and the
discussion in [studio/flamenco#104466][issue-104466].

[symlink-eval]: https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/fsutil-behavior
[symlink-policy]: https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-10/security/threat-protection/security-policy-settings/create-symbolic-links
[issue-104466]: https://projects.blender.org/studio/flamenco/issues/104466

#### Recommended setups with Windows Workers

| Manager + shared storage | Windows Workers? | What to configure |
| --- | --- | --- |
| Windows Manager, shared storage on a Windows network share | Yes, but not universally guaranteed | Create Symbolic Links permission on the Manager host, plus `R2R:1` on each Windows Worker |
| Linux Manager, shared storage on Linux/Samba | Yes | Two-way [storage variables]({{< ref "/usage/variables/multi-platform" >}}). Samba follows symlinks on the server, so Workers usually see normal files and do not need `R2R` |
| Mixed Windows and Linux Workers | Use Linux Manager with shared storage on Linux/Samba | A Windows Manager creates Windows reparse points, which Linux Workers cannot follow. A Linux Manager creates Unix symlinks; with Samba following them on the server, both Windows and Linux Workers typically see ordinary files |

Blender Studio does not run Flamenco on Windows day-to-day, so community
setups are how support improves. If something fails, please
[file a report][new-issue] or ask in
[the Flamenco channel on Blender Chat][flamenco-chat].
Other known configurations include putting `shared_storage_path` on
local NTFS for a single-machine test, or running the Manager on Linux. Shaman
remains disabled by default on Windows.

[flamenco-chat]: https://chat.blender.org/#/room/#flamenco:blender.org
[new-issue]: https://projects.blender.org/studio/flamenco/issues/new/choose

### Linux

For symlinks to work with CIFS/Samba filesystems (like a typical NAS), you need
to mount it with the option `mfsymlinks`. As a concrete example, for a user
`sybren`, put something like this in `fstab`:

```
//NAS/flamenco /media/flamenco cifs mfsymlinks,credentials=/home/sybren/.smbcredentials,uid=sybren,gid=users 0 0
```

Then put the NAS credentials in `/home/sybren/.smbcredentials`:

```
username=sybren
password=g1mm3acce55plz
```

and be sure to protect it with `chmod 600 /home/sybren/.smbcredentials`.

Finally `mkdir /media/flamenco` and `sudo mount /media/flamenco` should get things mounted.

The above info was obtained from [Ask Ubuntu](https://askubuntu.com/a/157140).

## Enabling Symlinks on SAMBA

If you're using SAMBA to host your Shared Storage, you'll also need to enable symlinks
on your `/etc/samba/smb.conf` file.

To do this you must add the `follow symlinks` and `wide links` options to your globals,
as exemplified below.

```
[global]
# Symlink Parameters
follow symlinks = yes
wide links = yes
unix extensions = no
allow insecure wide links = no
```

You may try adding these parameters to your share sub-section only instead,
if you need a more restricted configuration.

```
[global]
allow insecure wide links = yes
unix extensions = no

[share]
follow symlinks = yes
wide links = yes
```

This configuration has been tested with both Windows and Linux clients working together
over the same shared storage.

The above information was obtained from [UNIX Stack Exchange](https://unix.stackexchange.com/q/5120)

## Enabling or Disabling Shaman

Shaman is enabled by default on Linux and macOS. Since on Windows symbolic
links are not that commonly used, require extra system permissions (see
[Windows](#windows)), and clients on a shared network drive often need
[additional settings](#following-shaman-symlinks-over-a-shared-network-drive),
Shaman is disabled by default there.

To enable Shaman, edit `flamenco-manager.yaml` and set `shaman.enabled: true`
like this:

```yaml
shaman:
  enabled: true
```

Similarly, it can be disabled by setting it to `false`.

This is also accessible in the web interface. Click the settings cog in the
top right, toggle `Enable Shaman Storage`, and click Save.

{{< hint type=warning >}}

After changing this setting, be sure to **restart** Flamenco Manager, and
**refresh** the connection in the Blender add-on preferences. The last step is
necessary to make Blender fetch the updated configuration.

{{< /hint >}}

## Garbage Collection

Shaman keeps track of which files are still in use, and which files are not.
When a file in `file-store` is no longer symlinked from anywhere in the `jobs`
directory, it will automatically be deleted. When a job is submitted that
requires it, it will be reuploaded automatically.

The garbage collection system also keeps track of _when_ a file in `file-store`
is used by a job. Even when it's no longer symlinked (because, for example, you
cleaned up the `jobs` directory) it will only be removed 31 days after its last
use in a render job.

The garbage collector can be configured in `flamenco-manager.yaml`:

```yaml
shaman:
  enabled: true
  garbageCollect:
    period: 24h0m0s
    maxAge: 744h0m0s
```

- `period`: the garbage collector runs every 24 hours by default. Change this
  setting to make it more/less frequent.
- `maxAge`: unused files will only be removed when they haven't been referenced
  for this amount of time.
