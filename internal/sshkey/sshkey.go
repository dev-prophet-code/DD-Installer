// Package sshkey creates an SSH key pair for users who do not have one yet.
//
// This is the only place where the installer writes a credential to disk, and
// only when the user explicitly asks for it: the private key goes to the
// standard ~/.ssh folder (mode 0600), exactly like `ssh-keygen -t ed25519`.
// Existing files are never overwritten.
package sshkey

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"
)

// DefaultName is the file name used for keys created by the installer.
const DefaultName = "dd-installer_ed25519"

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Result describes a freshly created key pair.
type Result struct {
	PrivatePath string // e.g. /home/me/.ssh/dd-installer_ed25519
	PublicPath  string // PrivatePath + ".pub"
	PublicKey   string // one line, ready for authorized_keys
	Fingerprint string // SHA256:...
}

// DefaultDir returns ~/.ssh.
func DefaultDir() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".ssh"), nil
}

// Generate creates an Ed25519 key pair in dir (created with mode 0700 if
// missing). passphrase may be empty. It refuses to overwrite existing files.
func Generate(dir, name, passphrase, comment string) (Result, error) {
	if !nameRe.MatchString(name) {
		return Result{}, errors.New("key name may only contain letters, digits, '.', '_' and '-'")
	}
	if strings.ContainsAny(comment, "\r\n") {
		return Result{}, errors.New("invalid comment")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Result{}, err
	}
	priv := filepath.Join(dir, name)
	pub := priv + ".pub"
	for _, p := range []string{priv, pub} {
		if _, err := os.Lstat(p); err == nil {
			return Result{}, fmt.Errorf("%s already exists; choose another name (existing keys are never overwritten)", p)
		}
	}

	pk, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Result{}, err
	}
	var blk *pem.Block
	if passphrase != "" {
		blk, err = ssh.MarshalPrivateKeyWithPassphrase(sk, comment, []byte(passphrase))
	} else {
		blk, err = ssh.MarshalPrivateKey(sk, comment)
	}
	if err != nil {
		return Result{}, err
	}
	spk, err := ssh.NewPublicKey(pk)
	if err != nil {
		return Result{}, err
	}
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(spk)))
	if comment != "" {
		line += " " + comment
	}

	if err := writeExclusive(priv, pem.EncodeToMemory(blk), 0o600); err != nil {
		return Result{}, err
	}
	if err := writeExclusive(pub, []byte(line+"\n"), 0o644); err != nil {
		_ = os.Remove(priv) // do not leave a half-created pair behind
		return Result{}, err
	}
	return Result{PrivatePath: priv, PublicPath: pub, PublicKey: line, Fingerprint: ssh.FingerprintSHA256(spk)}, nil
}

func writeExclusive(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = os.Remove(path)
		return err
	}
	return f.Close()
}

// InstallCommands returns copy-paste commands that put the public key on the
// server, for the user's own computer. The key line is single-quoted safely.
func InstallCommands(publicKey, user, host string) map[string]string {
	q := "'" + strings.ReplaceAll(publicKey, "'", `'\''`) + "'"
	target := user + "@" + host
	return map[string]string{
		"macOS / Linux": "ssh-copy-id -i ~/.ssh/" + DefaultName + ".pub " + target,
		"Windows (PowerShell)": `type $env:USERPROFILE\.ssh\` + DefaultName + `.pub | ssh ` + target +
			` "mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys"`,
		"On the server itself": "mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo " + q + " >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys",
	}
}
