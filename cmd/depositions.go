package cmd

import (
	"fmt"
	"time"

	"github.com/kehao95/zenodo-cli/internal/errs"
	"github.com/kehao95/zenodo-cli/internal/zenodo"
	"github.com/spf13/cobra"
)

func (a *app) depositions() *cobra.Command {
	group := &cobra.Command{Use: "depositions", Aliases: []string{"drafts"}, Short: "Prepare and preview drafts; publish only through an explicit action", Args: cobra.NoArgs}
	get := &cobra.Command{Use: "get ID", Short: "Get your deposition, including metadata and files", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path, err := depositPath(args[0])
		if err != nil {
			return err
		}
		return a.request(cmd, "GET", path, nil, true)
	}}
	var createData string
	create := &cobra.Command{Use: "create", Short: "Create an unpublished draft (never publishes)", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		body := map[string]any{}
		if createData != "" {
			m, err := metadataInput(cmd, createData)
			if err != nil {
				return err
			}
			body["metadata"] = m
		}
		return a.request(cmd, "POST", "/deposit/depositions", body, true)
	}}
	create.Flags().StringVar(&createData, "metadata", "", "metadata JSON, @file or - for stdin; omit for an empty draft")
	var updateData string
	update := &cobra.Command{Use: "update ID", Short: "Replace draft metadata using a complete object", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path, err := depositPath(args[0])
		if err != nil {
			return err
		}
		if err = require(updateData, "--metadata"); err != nil {
			return err
		}
		m, err := metadataInput(cmd, updateData)
		if err != nil {
			return err
		}
		return a.request(cmd, "PUT", path, map[string]any{"metadata": m}, true)
	}}
	update.Flags().StringVar(&updateData, "metadata", "", "complete metadata JSON, @file or - (replacement, not patch)")
	del := &cobra.Command{Use: "delete ID", Short: "Delete an unpublished draft (--confirm ID)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path, err := depositPath(args[0])
		if err != nil {
			return err
		}
		return a.request(cmd, "DELETE", path, nil, true)
	}}
	reserve := &cobra.Command{Use: "reserve-doi ID", Short: "Reserve an unregistered DOI while preserving metadata", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path, err := depositPath(args[0])
		if err != nil {
			return err
		}
		u, err := a.client.URL(path)
		if err != nil {
			return err
		}
		if err = a.client.Check("PUT", u); err != nil {
			return err
		}
		if err = a.client.RequireToken(); err != nil {
			return err
		}
		result, err := a.client.JSON(cmd.Context(), "GET", path, nil)
		if err != nil {
			return err
		}
		m := zenodo.Object(zenodo.Object(result)["metadata"])
		if m == nil {
			return errs.New(1, "invalid_response", "deposition has no metadata object")
		}
		m["prereserve_doi"] = true
		return a.request(cmd, "PUT", path, map[string]any{"metadata": m}, true)
	}}
	preview := &cobra.Command{Use: "preview ID", Short: "Inspect draft readiness and browser preview link using GET only", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path, err := depositPath(args[0])
		if err != nil {
			return err
		}
		if err = a.client.RequireToken(); err != nil {
			return err
		}
		result, err := a.client.JSON(cmd.Context(), "GET", path, nil)
		if err != nil {
			return err
		}
		problems := validateMetadata(zenodo.Object(zenodo.Object(result)["metadata"]))
		// Browser UI requires login. Keep this separate from API self/latest_draft.
		origin := a.client.Base.Scheme + "://" + a.client.Base.Host
		return a.print(cmd, map[string]any{"deposition": result, "metadata_valid": len(problems) == 0, "validation_errors": problems, "edit_url": origin + "/uploads/" + args[0], "preview_url": origin + "/records/" + args[0] + "?preview=1", "note": "Browser preview requires login; local checks are advisory. Only publish makes the draft public."})
	}}
	group.AddCommand(a.listCommand("list", "/deposit/depositions", true), get, create, update, del, reserve, preview)
	for _, action := range []string{"publish", "edit", "discard", "newversion"} {
		action := action
		short := map[string]string{"publish": "Publish draft (--confirm ID; public DOI registration)", "edit": "Unlock published metadata for editing", "discard": "Discard edits (--confirm ID)", "newversion": "Create a new draft version and return source and draft"}[action]
		c := &cobra.Command{Use: action + " ID", Short: short, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			path, err := depositPath(args[0])
			if err != nil {
				return err
			}
			path += "/actions/" + action
			if action != "newversion" {
				return a.request(cmd, "POST", path, nil, true)
			}
			u, err := a.client.URL(path)
			if err != nil {
				return err
			}
			if err = a.client.Check("POST", u); err != nil {
				return err
			}
			if err = a.client.RequireToken(); err != nil {
				return err
			}
			source, err := a.client.JSON(cmd.Context(), "POST", path, nil)
			if err != nil {
				return err
			}
			link := zenodo.Link(source, "latest_draft")
			if link == "" {
				return errs.New(1, "invalid_response", "new version may have been created, but latest_draft is missing; inspect source before retrying")
			}
			draft, err := a.client.JSON(cmd.Context(), "GET", link, nil)
			if err != nil {
				return fmt.Errorf("new version created; fetch latest_draft to recover: %w", err)
			}
			return a.print(cmd, map[string]any{"source": source, "draft": draft})
		}}
		if action == "newversion" {
			c.Aliases = []string{"new-version"}
		}
		group.AddCommand(c)
	}
	return group
}

func validateMetadata(m map[string]any) []string {
	problems := []string{}
	for _, key := range []string{"title", "upload_type", "description"} {
		if zenodo.String(m[key]) == "" {
			problems = append(problems, key+" is required")
		}
	}
	creators, ok := m["creators"].([]any)
	if !ok || len(creators) == 0 {
		problems = append(problems, "at least one creator is required")
	} else {
		for i, v := range creators {
			if zenodo.String(zenodo.Object(v)["name"]) == "" {
				problems = append(problems, fmt.Sprintf("creators.%d.name is required", i))
			}
		}
	}
	for kind, key := range map[string]string{"publication": "publication_type", "image": "image_type"} {
		if m["upload_type"] == kind && zenodo.String(m[key]) == "" {
			problems = append(problems, key+" is required for "+kind)
		}
	}
	if date := zenodo.String(m["publication_date"]); date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			problems = append(problems, "publication_date must be YYYY-MM-DD")
		}
	}
	if m["access_right"] == "embargoed" && zenodo.String(m["embargo_date"]) == "" {
		problems = append(problems, "embargo_date is required")
	}
	if m["access_right"] == "restricted" && zenodo.String(m["access_conditions"]) == "" {
		problems = append(problems, "access_conditions is required")
	}
	return problems
}

func (a *app) metadata() *cobra.Command {
	group := &cobra.Command{Use: "metadata", Short: "Offline helpers for legacy Zenodo metadata", Args: cobra.NoArgs}
	template := &cobra.Command{Use: "template", Short: "Print an example metadata object", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return a.print(cmd, map[string]any{"title": "Research artifact", "upload_type": "dataset", "description": "Describe your artifact.", "creators": []any{map[string]any{"name": "Family, Given"}}, "access_right": "open", "license": "cc-by-4.0"})
	}}
	var spec string
	validate := &cobra.Command{Use: "validate", Short: "Check common requirements locally; Zenodo owns full validation", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if err := require(spec, "--metadata"); err != nil {
			return err
		}
		m, err := metadataInput(cmd, spec)
		if err != nil {
			return err
		}
		problems := validateMetadata(m)
		if len(problems) > 0 {
			return &errs.Error{Code: 2, Kind: "invalid_metadata", Message: "metadata is missing common publication requirements", Details: problems}
		}
		return a.print(cmd, map[string]any{"valid": true, "validation": "local common requirements only"})
	}}
	validate.Flags().StringVar(&spec, "metadata", "", "metadata JSON, @file or -")
	group.AddCommand(template, validate)
	return group
}
