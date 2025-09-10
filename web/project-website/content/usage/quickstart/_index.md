---
title: Quickstart
weight: 0
---

What you need to run Flamenco is:

- One or more computers to do the work, i.e. running **Flamenco Worker**. You'll
  probably also want to install [Blender][blender] there.
- A computer to run the central software, **Flamenco Manager**. This could be
  one of the above computers, or a dedicated one.
- A local network with **file sharing** already set up, so that the above
  computers can all reach the same set of files.

[blender]: https://www.blender.org/

In broad terms, to start rendering with Flamenco, follow the steps below.

1. [Download Flamenco][download].
2. Create a directory on some storage, like a NAS, and make sure it's available at the same path on each computer.
3. Install Blender on each computer you want to render on. It should be in the same place everywhere.
4. Pick the computer that will manage the farm. Run `flamenco-manager` on it. This will start a web browser with the *Flamenco Setup Assistant*.
5. Step through the assistant, pointing it to the storage (step 2) and Blender (step 3). Be sure to confirm at the final step.
6. Download the *Blender add-on* and install it. The link is in the top-right corner in your browser.
7. Configure the add-on by giving it the address of Flamenco Manager. You can see this in your web browser, and the Flamenco Manager logs also show URLs you can try. Be sure to click the checkmark to check the connection.
8. Save your Blend file in the shared storage.
9. Tell Flamenco to render it. You can find the Flamenco panel in Blender's Output Properties.

[download]: {{< ref "download" >}}

{{< hint type=info >}}

The above steps will get you started, but are not the only way to use Flamenco.
Continue reading about [shared storage][storage].

[storage]: {{< ref "usage/shared-storage" >}}
{{< /hint >}}

## Setup Flamenco in 5 minutes on Windows

{{< youtube O728EFaXuBk >}}

## Blender Conference 2022 presentation

In this presentation Sybren explains the use of Flamenco, and dives a bit deeper
into the storage options and the custom job types.

{{< youtube shIDWVSTGe4 >}}
