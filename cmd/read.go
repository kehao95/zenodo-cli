package cmd

import (
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/kehao95/zenodo-cli/internal/errs"
	"github.com/kehao95/zenodo-cli/internal/zenodo"
	"github.com/spf13/cobra"
)

type listOptions struct {
	query, sort, status, community, kind, subtype string
	page, size, maxPages                          int
	all, versions                                 bool
}

func (a *app) listCommand(use, path string, auth bool) *cobra.Command {
	o := &listOptions{}
	c := &cobra.Command{Use: use, Short: "List/search resources with pagination", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		maxSize := 100
		if path == "/records" && a.client.Token == "" {
			maxSize = 25
		}
		if o.page < 1 || o.size < 1 || o.size > maxSize || o.maxPages < 1 {
			return errs.New(2, "invalid_input", fmt.Sprintf("page/max-pages must be positive; size must be 1..%d", maxSize))
		}
		if o.status != "" && o.status != "draft" && o.status != "published" {
			return errs.New(2, "invalid_input", "status must be draft or published")
		}
		if auth {
			if err := a.client.RequireToken(); err != nil {
				return err
			}
		}
		q := url.Values{"page": {strconv.Itoa(o.page)}, "size": {strconv.Itoa(o.size)}}
		for k, v := range map[string]string{"q": o.query, "sort": o.sort, "status": o.status, "communities": o.community, "type": o.kind, "subtype": o.subtype} {
			if v != "" {
				q.Set(k, v)
			}
		}
		if o.versions {
			q.Set("all_versions", "true")
		}
		next := path + "?" + q.Encode()
		pages := []any{}
		seen := map[string]bool{}
		for count := 0; ; count++ {
			u, err := a.client.URL(next)
			if err != nil {
				return err
			}
			key := u.String()
			if seen[key] {
				return errs.New(1, "pagination_loop", "Zenodo returned a repeated next page")
			}
			seen[key] = true
			page, err := a.client.JSON(cmd.Context(), "GET", next, nil)
			if err != nil {
				return err
			}
			if !o.all {
				return a.print(cmd, page)
			}
			pages = append(pages, page)
			next = zenodo.Link(page, "next")
			// Deposition list is a native JSON array with page-number pagination.
			if items, ok := page.([]any); ok {
				if len(items) < o.size {
					next = ""
				} else {
					q.Set("page", strconv.Itoa(o.page+count+1))
					next = path + "?" + q.Encode()
				}
			}
			if next == "" {
				return a.print(cmd, map[string]any{"pages": pages, "complete": true})
			}
			if count+1 >= o.maxPages {
				return a.print(cmd, map[string]any{"pages": pages, "complete": false, "next": next})
			}
		}
	}}
	f := c.Flags()
	f.StringVarP(&o.query, "query", "q", "", "Zenodo search query")
	f.IntVar(&o.page, "page", 1, "start page")
	f.IntVar(&o.size, "size", 25, "page size (anonymous records: max 25; authenticated: max 100)")
	f.BoolVar(&o.all, "all", false, "collect pages into {pages,complete,next}; default bounded at 100 pages")
	f.IntVar(&o.maxPages, "max-pages", 100, "maximum pages with --all")
	if path != "/vocabularies/licenses" {
		f.StringVar(&o.sort, "sort", "", "Zenodo sort order")
		f.BoolVar(&o.versions, "all-versions", false, "include every version")
	}
	if auth {
		f.StringVar(&o.status, "status", "", "draft or published")
	}
	if path == "/records" {
		f.StringVar(&o.community, "community", "", "community identifier")
		f.StringVar(&o.kind, "type", "", "resource type")
		f.StringVar(&o.subtype, "subtype", "", "resource subtype")
	}
	return c
}

func (a *app) records() *cobra.Command {
	group := &cobra.Command{Use: "records", Short: "Published records: search, metadata, files, export and download", Args: cobra.NoArgs}
	search := a.listCommand("search", "/records", false)
	search.Aliases = []string{"list"}
	get := &cobra.Command{Use: "get ID", Aliases: []string{"info"}, Short: "Get record JSON", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		record, err := id(args[0])
		if err != nil {
			return err
		}
		return a.request(cmd, "GET", "/records/"+record, nil, false)
	}}
	files := &cobra.Command{Use: "files ID", Short: "List downloadable files from record metadata", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		record, err := id(args[0])
		if err != nil {
			return err
		}
		result, err := a.client.JSON(cmd.Context(), "GET", "/records/"+record, nil)
		if err != nil {
			return err
		}
		return a.print(cmd, zenodo.Object(result)["files"])
	}}
	var format string
	export := &cobra.Command{Use: "export ID", Short: "Write citation/metadata text to stdout", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		record, err := id(args[0])
		if err != nil {
			return err
		}
		formats := map[string]string{"bibtex": "application/x-bibtex", "datacite": "application/x-datacite+xml", "dc": "application/x-dc+xml", "marcxml": "application/marcxml+xml", "json": "application/json"}
		accept, ok := formats[format]
		if !ok {
			return errs.New(2, "invalid_input", "format must be bibtex, datacite, dc, marcxml, or json")
		}
		resp, err := a.client.Do(cmd.Context(), "GET", "/records/"+record, nil, "", accept)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
		return err
	}}
	export.Flags().StringVar(&format, "format", "bibtex", "export format (raw text stdout)")
	group.AddCommand(search, get, files, export, a.download("records"))
	return group
}

func (a *app) licenses() *cobra.Command {
	group := &cobra.Command{Use: "licenses", Short: "Search and inspect license vocabulary", Args: cobra.NoArgs}
	get := &cobra.Command{Use: "get ID", Short: "Get license metadata", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := segment(args[0]); err != nil {
			return err
		}
		return a.request(cmd, "GET", "/vocabularies/licenses/"+url.PathEscape(args[0]), nil, false)
	}}
	group.AddCommand(a.listCommand("list", "/vocabularies/licenses", false), get)
	return group
}
