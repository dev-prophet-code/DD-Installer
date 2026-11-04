# SSH guide for beginners

This guide explains how to connect DD-Installer to a **remote Linux server** over SSH, step by step, including how to create the **key file**. You do not need prior experience. Every command can be copied as it is.

> The in-app helpers do most of this for you: **Create SSH key** makes the key pair and shows the command for step 3, **SSH help** shows a short version of this guide.

## What is SSH and what do I need?

SSH is the secure way to log in to a Linux server from your computer. You need:

| Thing | What it is |
|---|---|
| **Server address** | The IP address (e.g. `203.0.113.10`) or name of your server. Your hosting provider shows it. |
| **A normal user with sudo** | A login that is **not** `root` but may run admin commands with `sudo`. DD-Installer refuses `root`. |
| **A key pair** (recommended) | Two files: the **private key** (stays on your computer, never share it) and the **public key** (goes onto the server). Like a lock (public) and the only matching key (private). |

You can also log in with a **password**; keys are safer and more convenient.

Where do I type commands?

| Your computer | Open this |
|---|---|
| Windows 10/11 | Start menu, type **PowerShell**, open *Windows PowerShell* (OpenSSH is built in) |
| macOS | **Terminal** (Spotlight: press Cmd+Space, type *Terminal*) |
| Linux | Your terminal (often Ctrl+Alt+T) |

Check that SSH exists: run `ssh -V`. If Windows says the command is unknown, install it under *Settings > Apps > Optional features > OpenSSH Client*.

## Step 1 - Make sure you have a normal user on the server

Many providers give you only `root` at first. Log in once as root and create a user. Replace `dune` with the name you like.

```bash
ssh root@YOUR-SERVER-IP
```

On the server:

```bash
adduser dune                 # asks for a password and some optional details
usermod -aG sudo dune        # lets the user run admin commands
exit
```

(On CentOS/Fedora/Rocky use `useradd -m dune && passwd dune && usermod -aG wheel dune`.)

If your provider already created a user for you (often `ubuntu` or `debian`), use that one.

## Step 2 - Create a key pair

### Option A: with DD-Installer (easiest)

1. Choose **Remote Linux server (SSH)**.
2. Click **Create SSH key**.
3. Optional: enter a **passphrase** (a password that protects the key if someone steals your files).
4. Click **Create key**. The key is saved in your `.ssh` folder and its path is filled in for you.

### Option B: with the terminal

The same on Windows (PowerShell), macOS and Linux:

```bash
ssh-keygen -t ed25519 -C "dd-installer"
```

* *Enter file in which to save the key*: press **Enter** to accept the default, or type a name.
* *Enter passphrase*: type one (recommended) or press Enter for none.

Result - two files in your `.ssh` folder:

| File | Meaning | Share it? |
|---|---|---|
| `id_ed25519` | **private key** | **Never** |
| `id_ed25519.pub` | **public key** | Yes, this goes to the server |

Where is the `.ssh` folder?

| OS | Path |
|---|---|
| Windows | `C:\Users\YOURNAME\.ssh` (type `explorer $env:USERPROFILE\.ssh` in PowerShell) |
| macOS | `/Users/YOURNAME/.ssh` (Finder: Cmd+Shift+G, type `~/.ssh`) |
| Linux | `/home/YOURNAME/.ssh` |

## Step 3 - Put the public key on the server

Run **one** of these on **your computer** (replace `dune` and `YOUR-SERVER-IP`; use `dd-installer_ed25519.pub` instead of `id_ed25519.pub` if you made the key with the app). You type the server password once.

**macOS / Linux**

```bash
ssh-copy-id -i ~/.ssh/id_ed25519.pub dune@YOUR-SERVER-IP
```

**Windows (PowerShell)** - there is no `ssh-copy-id`, use:

```powershell
type $env:USERPROFILE\.ssh\id_ed25519.pub | ssh dune@YOUR-SERVER-IP "mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys"
```

**Alternative - on the server itself** (e.g. in your provider's web console): log in as the user and run, with your own public key line between the quotes:

```bash
mkdir -p ~/.ssh && chmod 700 ~/.ssh
echo "ssh-ed25519 AAAA...your-public-key... dd-installer" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

**Hosting panels** (Hetzner, DigitalOcean, OVH, ...) usually have an *SSH keys* page where you paste the content of the `.pub` file *before* creating the server. Open the file with Notepad / TextEdit and copy the whole line.

## Step 4 - Test the connection

```bash
ssh -i ~/.ssh/id_ed25519 dune@YOUR-SERVER-IP
```

(Windows: `ssh -i $env:USERPROFILE\.ssh\id_ed25519 dune@YOUR-SERVER-IP`.) The first time you see *"The authenticity of host ... can't be established"* - type `yes` after checking the fingerprint (see below). You should land on the server's prompt. Type `exit` to leave. Also test that admin rights work: `sudo -v`.

## Step 5 - Fill in DD-Installer

| Field | What to enter |
|---|---|
| **Host** | The server IP or name |
| **Port** | `22` (unless your provider changed it) |
| **User** | `dune` (your normal user, **not root**) |
| **Authentication** | *Private key* |
| **Key file** | Path of the **private** key, e.g. `C:\Users\You\.ssh\id_ed25519` (use **Browse...**) |
| **Key passphrase** | The passphrase from step 2, or empty |

Press **Next**, go to **Scan**, press **Scan target**. On the first connection DD-Installer shows the server's **fingerprint**. Compare it with the real one before you press *Trust*: log in at your provider's web console and run

```bash
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

Both must be identical. This protects you from someone intercepting the connection. DD-Installer remembers the fingerprint only for the current session.

### Password instead of a key

Choose **Password**, enter your user's password. The password is also used for `sudo`. Some providers disable password login; then use a key.

### ssh-agent

If you use an agent (it keeps unlocked keys in memory), choose **ssh-agent**. Add the key first: `ssh-add ~/.ssh/id_ed25519`. On Windows start the agent once as administrator: `Get-Service ssh-agent | Set-Service -StartupType Automatic; Start-Service ssh-agent`.

## Troubleshooting

| Message | Likely cause and fix |
|---|---|
| `Connection refused` | SSH server not running or wrong port. On the server: `sudo systemctl enable --now ssh` (Ubuntu/Debian) or `sshd` (others). Check the port. |
| `Connection timed out` / `No route to host` | Wrong IP, server off, or a firewall blocks port 22. Check your provider's firewall / security group. |
| `Permission denied (publickey)` | Public key not installed for **that user** (repeat step 3), wrong user name, or wrong key file chosen. |
| `Permission denied` after typing a password | Wrong password or the server disabled password login. Use a key. |
| `UNPROTECTED PRIVATE KEY FILE` / `bad permissions` | Private key readable by others. macOS/Linux: `chmod 600 ~/.ssh/id_ed25519`. Windows: right-click the file > Properties > Security, keep only your own user. |
| `passphrase` / `incorrect passphrase` | Wrong key passphrase. If forgotten, create a new key (step 2). |
| `Host key changed` / `REMOTE HOST IDENTIFICATION HAS CHANGED` | The server was reinstalled - **or** someone is intercepting. Only continue if you know why; compare the fingerprint again. |
| DD-Installer says the user is root | Create a normal user (step 1) and use that. |
| `sudo: a password is required` | The assistant can enable temporary passwordless sudo if you entered the password; otherwise see [ASSISTANT.md](ASSISTANT.md). |

## Good habits

* Use a **passphrase** on your key and keep a backup of the private key in a safe place (password manager). If you lose it you can still use the provider's web console.
* **Never** send the private key (`id_ed25519`, no `.pub`) to anyone, not even support staff.
* Once keys work you can disable password login on the server (`PasswordAuthentication no` in `/etc/ssh/sshd_config`, then `sudo systemctl restart ssh`) - only do this after you tested the key.
