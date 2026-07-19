package commands

import "github.com/spf13/cobra"

// NewRoot builds the devsync CLI root command with all subcommands.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "devsync",
		Short:         "devsync - end-to-end encrypted credential store",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		// setup / identity
		newConfigCmd(),
		newInitCmd(),
		newRegisterCmd(),
		newBootstrapAdminCmd(),
		newWhoAmICmd(),
		newUnlockCmd(),
		// team / vault admin
		newCreateTeamCmd(),
		newInviteCmd(),
		newJoinCmd(),
		newTeamsCmd(),
		newMembersCmd(),
		newApproveCmd(),
		newCreateVaultCmd(),
		newGrantCmd(),
		newRevokeCmd(),
		// files
		newPushCmd(),
		newPullCmd(),
		newHistoryCmd(),
		newCheckoutCmd(),
		newRmCmd(),
		newAuditCmd(),
		// device
		newDeviceCmd(),
	)
	return root
}
