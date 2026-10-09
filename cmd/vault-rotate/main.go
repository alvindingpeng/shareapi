// Command vault-rotate re-encrypts all vaulted channel credentials from one
// KEK to another. It is an offline maintenance tool: stop every app server
// first, then run:
//
//	VAULT_KEK=<old base64 KEK> VAULT_KEK_NEW=<new base64 KEK> SQL_DSN=... vault-rotate
//
// On success, replace VAULT_KEK with the new value on all app servers and
// restart them. The operation is idempotent and safe to re-run after a crash:
// rows already stamped with the new KEK's ID are skipped, and a wrong old KEK
// aborts before touching any row.
package main

import (
	"fmt"
	"os"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/vault"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "vault-rotate: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	oldKEK, err := vault.ParseKEK(vault.EnvKEK, os.Getenv(vault.EnvKEK))
	if err != nil {
		return err
	}
	newKEK, err := vault.ParseKEK("VAULT_KEK_NEW", os.Getenv("VAULT_KEK_NEW"))
	if err != nil {
		return err
	}
	common.InitEnv()
	if err := model.InitDB(); err != nil {
		return fmt.Errorf("init DB: %w", err)
	}
	rotated, err := model.RotateChannelKEK(oldKEK, newKEK)
	if err != nil {
		return err
	}
	fmt.Printf("vault-rotate: re-encrypted %d channel(s); new kek_id %s\n", rotated, vault.KEKID(newKEK))
	fmt.Println("vault-rotate: now set VAULT_KEK to the new value on all app servers and restart them")
	return nil
}
