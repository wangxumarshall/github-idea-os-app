package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var ideaCmd = &cobra.Command{
	Use:   "idea",
	Short: "Work with ideas",
}

var ideaGetCmd = &cobra.Command{
	Use:   "get <slug>",
	Short: "Get idea details",
	Args:  exactArgs(1),
	RunE:  runIdeaGet,
}

func init() {
	ideaCmd.AddCommand(ideaGetCmd)
	ideaGetCmd.Flags().String("output", "json", "Output format: table or json")
}

func runIdeaGet(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var idea map[string]any
	if err := client.GetJSON(ctx, "/api/ideas/"+args[0], &idea); err != nil {
		return fmt.Errorf("get idea: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		headers := []string{"CODE", "TITLE", "REPO", "ROOT ISSUE"}
		rows := [][]string{{
			strVal(idea, "code"),
			strVal(idea, "title"),
			strVal(idea, "project_repo_url"),
			strVal(idea, "root_issue_id"),
		}}
		cli.PrintTable(os.Stdout, headers, rows)
		return nil
	}

	return cli.PrintJSON(os.Stdout, idea)
}
