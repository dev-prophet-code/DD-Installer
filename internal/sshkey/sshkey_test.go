package sshkey

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestGenerateAndParse(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".ssh")
	r, err := Generate(dir, DefaultName, "", "dd-installer")
	if err != nil {
		t.Fatal(err)
	}
	pem, _ := os.ReadFile(r.PrivatePath)
	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		t.Fatal(err)
	}
	if ssh.FingerprintSHA256(signer.PublicKey()) != r.Fingerprint {
		t.Fatal("fingerprint mismatch")
	}
	if !strings.HasPrefix(r.PublicKey, "ssh-ed25519 ") || !strings.HasSuffix(r.PublicKey, " dd-installer") {
		t.Fatalf("public key line: %q", r.PublicKey)
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(r.PrivatePath)
		if st.Mode().Perm() != 0o600 {
			t.Fatalf("private key mode %v", st.Mode().Perm())
		}
		ds, _ := os.Stat(dir)
		if ds.Mode().Perm() != 0o700 {
			t.Fatalf("dir mode %v", ds.Mode().Perm())
		}
	}
}

func TestPassphraseProtected(t *testing.T) {
	r, err := Generate(t.TempDir(), "k", "s3cret-pass", "c")
	if err != nil {
		t.Fatal(err)
	}
	pem, _ := os.ReadFile(r.PrivatePath)
	if _, err := ssh.ParsePrivateKey(pem); err == nil {
		t.Fatal("encrypted key must need a passphrase")
	}
	if _, err := ssh.ParsePrivateKeyWithPassphrase(pem, []byte("s3cret-pass")); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(pem), "s3cret") {
		t.Fatal("passphrase leaked into key file")
	}
}

func TestNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(dir, "k", "", ""); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "k"))
	if _, err := Generate(dir, "k", "", ""); err == nil {
		t.Fatal("must refuse to overwrite")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "k"))
	if string(before) != string(after) {
		t.Fatal("existing key was modified")
	}
}

func TestRejectsBadNames(t *testing.T) {
	for _, n := range []string{"../evil", "a/b", `a\b`, "", ".hidden", "a b", strings.Repeat("a", 70)} {
		if _, err := Generate(t.TempDir(), n, "", ""); err == nil {
			t.Errorf("name %q accepted", n)
		}
	}
	if _, err := Generate(t.TempDir(), "ok", "", "x\ny"); err == nil {
		t.Error("newline in comment accepted")
	}
}

func TestInstallCommandsQuoting(t *testing.T) {
	c := InstallCommands("ssh-ed25519 AAAA c'om", "dune", "1.2.3.4")
	if !strings.Contains(c["macOS / Linux"], "ssh-copy-id") || !strings.Contains(c["Windows (PowerShell)"], "USERPROFILE") {
		t.Fatal(c)
	}
	if strings.Contains(c["On the server itself"], "c'om") && !strings.Contains(c["On the server itself"], `'\''`) {
		t.Fatal("quote not escaped")
	}
}
