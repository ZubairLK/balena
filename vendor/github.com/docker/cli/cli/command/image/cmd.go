package image

import (
	"github.com/spf13/cobra"

	"github.com/docker/cli/cli"
	"github.com/docker/cli/cli/command"
)

// NewImageCommand returns a cobra command for `image` subcommands
// nolint: interfacer
func NewImageCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Manage images",
		Args:  cli.NoArgs,
		RunE:  command.ShowHelp(dockerCli.Err()),
	}
	cmd.AddCommand(
		NewBuildCommand(dockerCli),
		NewHistoryCommand(dockerCli),
		NewDeltaCommand(dockerCli),
		NewImportCommand(dockerCli),
		NewLoadCommand(dockerCli),
		NewPullCommand(dockerCli),
		NewPushCommand(dockerCli),
		NewSaveCommand(dockerCli),
		NewTagCommand(dockerCli),
		newListCommand(dockerCli),
		newRemoveCommand(dockerCli),
		newInspectCommand(dockerCli),
		NewPruneCommand(dockerCli),
	)
	return cmd
}
