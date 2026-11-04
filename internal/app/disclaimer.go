package app

// DisclaimerShort is shown permanently in the UI.
const DisclaimerShort = "Unofficial community tool. DD-Installer does NOT store your credentials (SSH password, key passphrase, Funcom token stay in memory for this session only). " +
	"Neither RedBlink (author of Dune: Awakening Docker) nor Prophet (author of DD-Installer) can be held liable for any damage."

// DisclaimerFull mirrors docs/DISCLAIMER.md.
const DisclaimerFull = `DISCLAIMER

DD-Installer is an unofficial community tool. It is not affiliated with, endorsed by or supported by Funcom, nor by RedBlink, the author of "Dune: Awakening Docker".

CREDENTIALS
- DD-Installer does not store credentials. SSH passwords, key passphrases and the Funcom token are held in memory for the running session only. They are never written to disk, log files or configuration files by this tool and are sent only to the server you selected.
- Neither RedBlink nor Prophet (author of DD-Installer) receives, collects or stores your credentials.
- The installed Dune Docker software itself stores the Funcom token and the generated admin password in files on your own server (runtime/secrets/). That is upstream behaviour under your control.
- If you approve the assistant's temporary passwordless sudo fix, it is removed again when the run ends. If the installer is killed earlier, delete /etc/sudoers.d/90-dd-installer manually.

NO WARRANTY, NO LIABILITY
The software is provided "as is", without warranty of any kind. It installs software (Docker, packages, on Windows also WSL2 and Ubuntu), changes system settings and starts game servers on the machine you choose. You are responsible for backups and for reviewing what it does.

Neither RedBlink nor Prophet, nor any contributor, can be held liable or held responsible for any damage, data loss, downtime, security incident, cost or other consequence arising from the use of DD-Installer or Dune Docker, to the extent permitted by law.`
