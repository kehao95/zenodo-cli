package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/kehao95/zenodo-cli/internal/errs"
	"github.com/kehao95/zenodo-cli/internal/zenodo"
	"github.com/spf13/cobra"
)

func segment(s string) error {
	if strings.TrimSpace(s) == "" || s == "." || s == ".." || strings.ContainsAny(s, "/\\\x00\r\n") {
		return errs.New(2, "invalid_input", "expected a nonempty filename/identifier without path separators")
	}
	return nil
}

func (a *app) files() *cobra.Command {
	group := &cobra.Command{Use: "files", Short: "Manage files of your deposition (upload never publishes)", Args: cobra.NoArgs}
	list := &cobra.Command{Use: "list DEPOSITION", Short: "List deposition files", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p, err := depositPath(args[0])
		if err != nil {
			return err
		}
		return a.request(cmd, "GET", p+"/files", nil, true)
	}}
	get := &cobra.Command{Use: "get DEPOSITION FILE_ID", Short: "Get file metadata by file ID", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		p, err := depositPath(args[0])
		if err != nil {
			return err
		}
		if err = segment(args[1]); err != nil {
			return err
		}
		return a.request(cmd, "GET", p+"/files/"+url.PathEscape(args[1]), nil, true)
	}}
	del := &cobra.Command{Use: "delete DEPOSITION FILE_ID", Short: "Delete draft file (--confirm DEPOSITION)", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		p, err := depositPath(args[0])
		if err != nil {
			return err
		}
		if err = segment(args[1]); err != nil {
			return err
		}
		return a.request(cmd, "DELETE", p+"/files/"+url.PathEscape(args[1]), nil, true)
	}}
	rename := &cobra.Command{Use: "rename DEPOSITION FILE_ID NAME", Short: "Rename a deposition file", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		p, err := depositPath(args[0])
		if err != nil {
			return err
		}
		for _, s := range args[1:] {
			if err = segment(s); err != nil {
				return err
			}
		}
		return a.request(cmd, "PUT", p+"/files/"+url.PathEscape(args[1]), map[string]any{"name": args[2]}, true)
	}}
	order := &cobra.Command{Use: "sort DEPOSITION FILE_ID...", Short: "Set file order (first file is the default preview)", Args: cobra.MinimumNArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		p, err := depositPath(args[0])
		if err != nil {
			return err
		}
		items := []any{}
		seen := map[string]bool{}
		for _, s := range args[1:] {
			if err = segment(s); err != nil {
				return err
			}
			if seen[s] {
				return errs.New(2, "invalid_input", "duplicate file ID")
			}
			seen[s] = true
			items = append(items, map[string]any{"id": s})
		}
		return a.request(cmd, "PUT", p+"/files", items, true)
	}}
	var name string
	var replace bool
	upload := &cobra.Command{Use: "upload DEPOSITION PATH", Short: "Stream a file into the draft's bucket; refuse name collisions by default", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		p, err := depositPath(args[0])
		if err != nil {
			return err
		}
		n := name
		if n == "" {
			n = filepath.Base(args[1])
		}
		if err = segment(n); err != nil {
			return err
		}
		u, err := a.client.URL(p + "/files")
		if err != nil {
			return err
		}
		if err = a.client.Check("PUT", u); err != nil {
			return err
		}
		info, err := os.Stat(args[1])
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errs.New(2, "invalid_input", "upload source must be a regular file")
		}
		if err = a.client.RequireToken(); err != nil {
			return err
		}
		deposit, err := a.client.JSON(cmd.Context(), "GET", p, nil)
		if err != nil {
			return err
		}
		if !replace {
			for _, file := range zenodo.FileEntries(zenodo.Object(deposit)["files"]) {
				if zenodo.FileName(file) == n {
					return errs.New(2, "file_exists", "remote filename exists; use --replace to overwrite")
				}
			}
		}
		bucket := zenodo.Link(deposit, "bucket")
		if bucket == "" {
			return errs.New(1, "invalid_response", "deposition has no bucket link")
		}
		target := strings.TrimRight(bucket, "/") + "/" + url.PathEscape(n)
		result, err := a.client.Upload(cmd.Context(), target, args[1])
		if err != nil {
			return err
		}
		return a.print(cmd, result)
	}}
	upload.Flags().StringVar(&name, "name", "", "remote filename (default: local basename)")
	upload.Flags().BoolVar(&replace, "replace", false, "allow replacing a file with the same name")
	group.AddCommand(list, get, del, rename, order, upload, a.download("depositions"))
	return group
}

func (a *app) download(resource string) *cobra.Command {
	var output string
	var force bool
	c := &cobra.Command{Use: "download ID NAME", Short: "Download a named file atomically; verify server checksum", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		record, err := id(args[0])
		if err != nil {
			return err
		}
		if err = segment(args[1]); err != nil {
			return err
		}
		p := "/records/" + record
		auth := resource == "depositions"
		if auth {
			p = "/deposit/depositions/" + record
		}
		dest := output
		if dest == "" {
			dest = args[1]
		}
		if auth {
			if err = a.client.RequireToken(); err != nil {
				return err
			}
		}
		result, err := a.client.JSON(cmd.Context(), "GET", p, nil)
		if err != nil {
			return err
		}
		for _, file := range zenodo.FileEntries(zenodo.Object(result)["files"]) {
			if zenodo.FileName(file) == args[1] {
				link := zenodo.Link(file, "content")
				if link == "" {
					link = zenodo.Link(file, "download")
				}
				if link == "" {
					link = zenodo.Link(file, "self")
				}
				if link == "" {
					return errs.New(1, "invalid_response", "file has no download link")
				}
				downloaded, err := a.client.Download(cmd.Context(), link, dest, zenodo.String(file["checksum"]), force)
				if err != nil {
					return err
				}
				return a.print(cmd, downloaded)
			}
		}
		return errs.New(7, "not_found", fmt.Sprintf("file %q was not found", args[1]))
	}}
	c.Flags().StringVarP(&output, "output", "o", "", "destination path (default: filename)")
	c.Flags().BoolVar(&force, "force", false, "replace an existing local file after successful checksum verification")
	return c
}
