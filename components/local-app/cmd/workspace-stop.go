// Copyright (c) 2023 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License.AGPL.txt in the project root for license information.

package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bufbuild/connect-go"
	v1 "github.com/gitpod-io/gitpod/components/public-api/go/experimental/v1"
	"github.com/gitpod-io/local-app/pkg/common"
	"github.com/spf13/cobra"
)

var stopDontWait = false

// stopWorkspaceCommand stops to a given workspace
var stopWorkspaceCommand = &cobra.Command{
	Use:   "stop <workspace-id>",
	Short: "Stop a given workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workspaceID := ""
		if len(args) < 1 {
			filter := func(ws *v1.Workspace) bool {
				return ws.GetStatus().Instance.Status.Phase != v1.WorkspaceInstanceStatus_PHASE_STOPPED
			}
			workspaceID = common.SelectWorkspace(cmd.Context(), filter)
		} else {
			workspaceID = args[0]
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()

		gitpod, err := common.GetGitpodClient(ctx)
		if err != nil {
			return err
		}

		fmt.Println("Attempting to stop workspace...")
		wsInfo, err := gitpod.Workspaces.StopWorkspace(ctx, connect.NewRequest(&v1.StopWorkspaceRequest{WorkspaceId: workspaceID}))
		if err != nil {
			return err
		}

		currentPhase := wsInfo.Msg.GetResult().Status.Instance.Status.Phase

		if currentPhase == v1.WorkspaceInstanceStatus_PHASE_STOPPED {
			fmt.Println("Workspace is already stopped")
			return nil
		}

		if currentPhase == v1.WorkspaceInstanceStatus_PHASE_STOPPING {
			fmt.Println("Workspace is already stopping")
			return nil
		}

		if stopDontWait {
			fmt.Println("Workspace stopping")
			return nil
		}

		stream, err := gitpod.Workspaces.StreamWorkspaceStatus(ctx, connect.NewRequest(&v1.StreamWorkspaceStatusRequest{WorkspaceId: workspaceID}))

		if err != nil {
			return err
		}

		fmt.Println("Waiting for workspace to stop...")

		fmt.Println("Workspace " + common.TranslateWsPhase(currentPhase.String()))

		previousStatus := ""

		for stream.Receive() {
			msg := stream.Msg()
			if msg == nil {
				fmt.Println("No message received")
				continue
			}

			if msg.GetResult().Instance.Status.Phase == v1.WorkspaceInstanceStatus_PHASE_STOPPED {
				fmt.Println("Workspace stopped")
				break
			}

			currentStatus := common.TranslateWsPhase(msg.GetResult().Instance.Status.Phase.String())

			if currentStatus != previousStatus {
				fmt.Println("Workspace " + currentStatus)
				previousStatus = currentStatus
			}
		}

		if err := stream.Err(); err != nil {
			log.Fatalf("Failed to receive: %v", err)
			return err
		}

		return nil
	},
}

func init() {
	workspaceCmd.AddCommand(stopWorkspaceCommand)
	stopWorkspaceCommand.Flags().BoolVarP(&stopDontWait, "dont-wait", "d", false, "do not wait for workspace to fully stop, only initialize")
}
