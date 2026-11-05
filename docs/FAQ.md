# FAQ

## Getting started

**What do I need?**
A Funcom self-host service token, a Linux server (Ubuntu recommended) with a normal user that may use `sudo` - or Windows with WSL2 / Linux on the same computer. The server needs an x86-64 CPU with AVX2, at least 20 GB RAM (30-40 GB with more Sietches or Deep Desert) and 200 GB disk. See [SIZING.md](SIZING.md).

**Where do I get the Funcom token?**
It belongs to your Funcom self-hosting account; follow Funcom's official self-hosting instructions to generate it. DD-Installer cannot create it for you. It must match the battlegroup you want to run (see the [upstream docs](https://docs.dunedocker.app/getting-started/first-run)).

**How long does it take?**
Usually 30-90 minutes. Most of that is the game download, which depends on your server's bandwidth.

**Can I install on a Mac?**
You can use a Mac to install on a **remote Linux server** over SSH. The server software itself needs x86-64 Linux with AVX2, which macOS cannot provide, so "this computer" is not offered on macOS.

**Does it work on Windows Home?**
Yes. WSL2 works on Windows 10/11 Home. If WSL2 or Ubuntu is missing, DD-Installer offers to install them (one administrator prompt, possibly one restart).

**Why is "root" refused?**
The upstream installer refuses to run as root, for good reasons. Create a normal user with sudo (see the [SSH guide](SSH-GUIDE.md), step 1).

## Safety and privacy

**Does DD-Installer store my passwords or my token?**
No. SSH password, key passphrase and Funcom token live in memory while the window is open; "Start over" clears them. The only things written to your computer are an SSH key pair you explicitly create with the key helper, and the non-secret "Effects on/off" preference. Details: [DISCLAIMER.md](DISCLAIMER.md), [SECURITY.md](SECURITY.md).

**Why does Windows / macOS warn me when I start it?**
The programs are not code-signed yet (signing certificates cost money). On Windows choose *More info > Run anyway*; on macOS right-click the app > *Open* (or `xattr -d com.apple.quarantine DD-Installer.app`). Verify your download with `SHA256SUMS`.

**What does the assistant change on my server?**
Only fixes for known problems, grouped by risk. *Safe* ones (install missing tools, wait for a package lock, pick another port, ...) run automatically. *Moderate* ones (docker group, temporary sudo) need your click. It never deletes data. See [ASSISTANT.md](ASSISTANT.md).

## During the installation

**Can I close the window or let the computer sleep?**
The long install step runs on the server and survives a closed window or a dropped connection. DD-Installer also keeps your computer awake while it works. If it was interrupted, start it again and press **Install**: finished steps are detected and skipped, a running deployment is re-attached.

**What does "Press Install again to continue" mean?**
A step failed. Read the explanation in the assistant panel, fix the cause (or let the assistant do it), then press **Install** again. Nothing is repeated that already succeeded.

**Which Dune Docker version is installed?**
The version DD-Installer was tested with (shown on the Server page and in the summary). You can choose the newest upstream release instead, at your own risk.

**What is the web console and where is the admin password?**
Dune Docker's browser-based admin panel. The Finish page shows its address and the generated admin password (also stored on the server in `runtime/secrets/admin-web-password.txt`). Never expose the console to untrusted users; see [ROUTER-AND-FIREWALL.md](ROUTER-AND-FIREWALL.md).

## Later

**How do I update or back up the server?**
Use **Manage existing server...** on the Target page: status, backup, game update, console update, restart. Or use the web console / `dune` CLI on the server.

**How do I remove everything?**
*Manage existing server... > Stop everything (keep all data)* stops it but keeps your files and databases. *Remove everything including data* deletes the folder and the databases and needs the typed phrase `DELETE ALL`. Download a backup first.

**Players cannot connect.**
Open the ports in your router and every firewall between the server and the internet: [ROUTER-AND-FIREWALL.md](ROUTER-AND-FIREWALL.md). Test from outside your home network.

**The animations make my computer slow / I do not like them.**
Click **Effects: on** in the top right to switch every animation off (remembered), or start with `DD_INSTALLER_NO_EFFECTS=1`.

More help: [TROUBLESHOOTING.md](TROUBLESHOOTING.md), the [official documentation](https://docs.dunedocker.app/) and the Dune Docker community.
