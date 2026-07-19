package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var helpTopics = map[string]string{
	"setup": `devsync setup — first-time wizard

  1. devsync init                  generate keypair
  2. devsync register --username x register with server
  3. devsync bootstrap-admin       become admin (first user only)
  4. devsync unlock                unlock device key
  5. devsync create-team <name>    create a team
  6. devsync create-vault <name>   create a vault
     --team <team>
  7. devsync push <file>           encrypt + upload
     --vault <vault>

Run 'devsync setup' for an interactive walkthrough.`,

	"team": `devsync team — sharing with others

  INVITE (admin adds user to team):
    devsync invite <user> --team <team>

  JOIN (user requests access):
    devsync join <team>

  APPROVE (admin activates user):
    devsync approve <user> --fingerprint <fp>

  GRANT (admin gives vault access):
    devsync grant <user> --vault <vault>

  Full flow:
    devsync approve budi --fingerprint SHA256:xxxx
    devsync invite budi --team eng
    devsync grant budi --vault secrets`,

	"device": `devsync device — manage devices

  LIST:
    devsync device list

  ADD a new device (same user):
    1. On new device: devsync init + devsync register --username <you>
    2. Admin approves: devsync approve <you> --fingerprint <fp>

  REVOKE a lost device:
    devsync device revoke <device-id>`,

	"security": `devsync security model

  - Ed25519 signed requests (anti-replay, ±5 min window)
  - X25519 sealed vault keys (per-device)
  - SecretBox encrypted blobs (XSalsa20-Poly1305)
  - Argon2id key derivation (t=3, m=64MB)
  - Server is zero-knowledge: ciphertext + sealed keys only
  - Revoke rotates vault key + re-encrypts all files`,
}

func newGuideCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guide <topic>",
		Short: "Show help for a specific topic",
		Long: `Topics: setup, team, device, security
Use 'devsync guide <topic>' to get detailed guidance.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			topic, ok := helpTopics[args[0]]
			if !ok {
				return fmt.Errorf("unknown topic %q — use: setup, team, device, security", args[0])
			}
			fmt.Println(topic)
			return nil
		},
	}
}
