---
title: Running as Background Service
weight: 10
---

Flamenco can be configured to run in the background through system services.
The service configuration is different for each operating system. This page will
guide you through a basic working example for each. From there, you can expand
it as needed.

{{< hint type=info >}}

**macOS and Windows:** Service configuration guides for these platforms are going
to be included later.

{{< /hint >}}

Please refer to the [Quickstart]({{< ref "usage/quickstart" >}}) if you haven't
already downloaded and set up Flamenco.

## Linux

Linux has many different init systems, such as systemd, OpenRC, runit and others.
For simplicity this guide will only cover **systemd**, as it is widely used across
all popular Linux distributions.

### Flamenco Worker

1. Copy the **flamenco-worker** binary to your shared storage or to your preferred
   install location:

   ```bash
   cp flamenco-worker /mnt/shared
   ```

   {{< hint type=tip >}}

   Keeping the worker binary on a shared storage simplifies [Upgrading]({{< ref "usage/upgrading" >}}),
   as you would only need to replace the binary in a single place.

   {{< /hint >}}

2. Create or use a dedicated **non-privileged user** to run the service:

   ```bash
   sudo useradd --system --user-group --create-home flamenco
   ```

   Then create the flamenco local data directory:

   ```bash
   sudo -u flamenco mkdir -p /home/flamenco/.local/share/flamenco
   ```

   {{< hint type=note >}}

   If using a different user, replace `-u flamenco` and `/home/flamenco/` with
   that user's name and home directory.

   {{< /hint >}}

3. Create the systemd unit file:

   ```bash
   sudo tee /etc/systemd/system/flamenco-worker.service <<EOT
   [Unit]
   Description=Flamenco Worker
   RequiresMountsFor=/mnt/shared

   [Service]
   ExecStart=/mnt/shared/flamenco-worker -restart-exit-code 47
   WorkingDirectory=/home/flamenco/.local/share/flamenco

   User=flamenco
   Group=flamenco

   RestartPreventExitStatus=SIGUSR1 SIGUSR2
   RestartForceExitStatus=47
   Restart=on-failure
   RestartSec=1s

   [Install]
   WantedBy=multi-user.target
   EOT
   ```

   Update the following fields as needed:

   - **`RequiresMountsFor=`** Path to your shared storage mount.

   - **`ExecStart=`** Full path to the `flamenco-worker` binary.

   - **`WorkingDirectory=`** Path to flamenco local data directory inside the user home directory.

   - **`User=`** and **`Group=`** Username and group of the user running the service.

   {{< hint type=tip >}}

   You can open the service unit in your default text editor using:

   ```bash
   sudo systemctl edit --full flamenco-worker
   ```

   This opens the full unit file for editing and automatically reloads it when saved.

   {{< /hint >}}

4. Enable and start the service:

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now flamenco-worker.service
   ```

5. Verify and inspect logs:

   ```bash
   systemctl status flamenco-worker.service
   ```

   View the full logs:

   ```bash
   journalctl -u flamenco-worker.service
   ```

   Follow live logs:

   ```bash
   journalctl -f -u flamenco-worker.service
   ```

### Flamenco Manager

{{< hint type=info >}}

If you have already created the `flamenco` user, use it here.

The same tip for editing with `systemctl edit` applies here as well.

{{< /hint >}}

1. Copy the **flamenco-manager** binary to your preferred install location:

   ```bash
   sudo mkdir /opt/flamenco
   sudo cp flamenco-manager /opt/flamenco/
   ```

   Set ownership:

   ```bash
   sudo chown -R flamenco:flamenco /opt/flamenco
   ```

2. Create the systemd unit file:

   ```bash
   sudo tee /etc/systemd/system/flamenco-manager.service <<EOT
   [Unit]
   Description=Flamenco Manager
   Documentation=https://flamenco.blender.org/
   After=network.target
   RequiresMountsFor=/mnt/shared

   [Service]
   ExecStart=/opt/flamenco/flamenco-manager
   WorkingDirectory=/opt/flamenco

   User=flamenco
   Group=flamenco

   Restart=on-failure
   RestartSec=1s

   [Install]
   WantedBy=multi-user.target
   EOT
   ```

   Update the following fields as needed:

   - **`RequiresMountsFor=`** Path to your shared storage mount.

   - **`ExecStart=`** Full path to the `flamenco-manager` binary.

   - **`WorkingDirectory=`** Path to directory containing the binary.

   - **`User=`** and **`Group=`** Username and group of the user running the service.

3. Enable and start the service:

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now flamenco-manager.service
   ```

4. Verify and inspect logs:

   ```bash
   systemctl status flamenco-manager.service
   ```

   View the full logs:

   ```bash
   journalctl -u flamenco-manager.service
   ```

   Follow live logs:

   ```bash
   journalctl -f -u flamenco-manager.service
   ```

### Services Hardening

After setting up and using the basic configuration in this guide, you might want
to harden your services to make them more robust and secure.

Here are a few properties that you can add under the **`[Service]`** section:

```ini
[Service]
# Ensures that the service and any of its child processes cannot elevate their privileges.
NoNewPrivileges=true

# Creates an isolated `tmp` directory for the service.
PrivateTmp=true

# Makes the OS filesystem read-only for the service.
ProtectSystem=strict

# Required when using ProtectSystem to allow write access
ReadWritePaths=/mnt/shared /opt/flamenco
```

### Troubleshooting

Most issues you might encounter are going to be from file permissions or incorrect paths.
If a service fails to start or behave correctly, your first step should always be to check the logs.

Here are the most common issues:

1. **Service Fails with `code=exited`**

   This error means systemd could not execute the Flamenco binary. The path in
   `ExecStart=` might be incorrect, or something else is preventing its execution.

   Try running the exact path in your unit file manually:

   ```bash
   sudo -u flamenco /opt/flamenco/flamenco-manager
   # or
   sudo -u flamenco /mnt/shared/flamenco-worker
   ```

2. **"Permission Denied" errors in the logs**

   The user running the service does not have ownership or read/write permissions
   to the `WorkingDirectory` or shared storage `(/mnt/shared)`.

   Try creating a file manually:

   ```bash
   sudo -u flamenco touch /opt/flamenco/test.txt
   sudo -u flamenco touch /mnt/shared/test.txt
   ```

3. **`blender` executable file not found in $PATH**

   If the worker service starts running but logs this error, it means that `blender`
   cannot be reached through the service's execution environment.

   Update `flamenco-worker.service` under the **`[Service]`** key:

   ```ini
   ExecSearchPath=/mnt/hdd/shared/blender:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/bin
   ```

   This defines the directories that systemd searches when resolving the
   `blender` executable.

4. **Unable to locate executable '/mnt/shared/flamenco-worker': Permission denied**

   If your distribution uses SELinux (or another security architecture), it may prevent
   the service from executing binaries.

   Try temporarily setting it to Permissive mode:

   ```bash
   setenforce 0
   ```

   If this resolves the issue, update your SELinux policy to allow the service.
