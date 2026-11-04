# Disclaimer

DD-Installer is an **unofficial community tool**. It is not affiliated with, endorsed by or supported by Funcom, nor by **RedBlink**, the author of *Dune: Awakening Docker* (Red-Blink/dune-awakening-selfhost-docker).

## Credentials

* DD-Installer **does not store credentials**. SSH passwords, key passphrases and the Funcom token are held in memory for the running session only. They are never written to disk, log files or configuration files by this tool and are sent only to the server you selected.
* **Neither RedBlink nor Prophet** (author of DD-Installer) receives, collects or stores your credentials.
* The optional **Create SSH key** helper writes a new key pair into your own `~/.ssh` folder, but only when you click it (private key with owner-only permissions; existing keys are never overwritten). It is your key, stored only there and never sent anywhere.
* The *installed* Dune Docker software stores the Funcom token and the generated admin password in files on your own server (`runtime/secrets/`). That is upstream behaviour under your control.
* If you approve the assistant's *temporary passwordless sudo* fix, it is removed again when the run ends. If the installer is killed earlier, delete `/etc/sudoers.d/90-dd-installer` manually.

## No warranty, no liability

The software is provided **"as is"**, without warranty of any kind. It installs software (Docker, packages, on Windows also WSL2 and Ubuntu), changes system settings and starts game servers on the machine you choose. You are responsible for backups and for reviewing what it does.

**Neither RedBlink nor Prophet, nor any contributor, can be held liable or held responsible** for any damage, data loss, downtime, security incident, cost or other consequence arising from the use of DD-Installer or Dune Docker, to the extent permitted by law.

The application requires you to accept this disclaimer before an installation can start.
