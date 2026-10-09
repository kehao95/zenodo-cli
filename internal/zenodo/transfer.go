package zenodo

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kehao95/zenodo-cli/internal/errs"
)

// MD5 is used only for Zenodo's integrity checks, not cryptographic security.
func VerifyChecksum(expected string, h hash.Hash) error {
	if expected == "" {
		return nil
	}
	expected = strings.TrimPrefix(expected, "md5:")
	expected = strings.TrimPrefix(expected, "sha256:")
	if !strings.EqualFold(expected, hex.EncodeToString(h.Sum(nil))) {
		return errs.New(1, "checksum_mismatch", "file checksum does not match Zenodo metadata")
	}
	return nil
}

func (c *Client) Download(ctx context.Context, link, destination, checksum string, force bool) (any, error) {
	if _, err := c.URL(link); err != nil {
		return nil, err
	}
	if !force {
		if _, err := os.Lstat(destination); err == nil {
			return nil, errs.New(2, "file_exists", "output exists; use --force to replace it")
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	resp, err := c.Do(ctx, "GET", link, nil, "", "*/*")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".zenodo-download-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	var h hash.Hash = md5.New()
	if strings.HasPrefix(checksum, "sha256:") {
		h = sha256.New()
	} else if strings.Contains(checksum, ":") && !strings.HasPrefix(checksum, "md5:") {
		return nil, errs.New(2, "unsupported_checksum", "unsupported checksum algorithm")
	}
	n, err := io.Copy(io.MultiWriter(tmp, h), resp.Body)
	if err != nil {
		return nil, err
	}
	if err = VerifyChecksum(checksum, h); err != nil {
		return nil, err
	}
	if err = tmp.Sync(); err != nil {
		return nil, err
	}
	if err = tmp.Close(); err != nil {
		return nil, err
	}
	if force {
		err = os.Rename(tmp.Name(), destination)
	} else {
		err = os.Link(tmp.Name(), destination)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "output": destination, "size": n, "checksum_verified": checksum != ""}, nil
}

type sizedReader struct {
	io.Reader
	size int64
}

func (r sizedReader) Size() int64 { return r.size }

func (c *Client) Upload(ctx context.Context, link, path string) (any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errs.New(2, "invalid_input", "upload source must be a regular file")
	}
	h := md5.New()
	resp, err := c.Do(ctx, "PUT", link, sizedReader{Reader: io.TeeReader(f, h), size: info.Size()}, "application/octet-stream", "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	dec.UseNumber()
	var result any
	if err = dec.Decode(&result); err != nil {
		return nil, errs.New(1, "invalid_response", "upload may have succeeded but response is invalid; inspect files before retrying")
	}
	if checksum := String(Object(result)["checksum"]); checksum != "" {
		if err = VerifyChecksum(checksum, h); err != nil {
			return nil, fmt.Errorf("uploaded file needs inspection: %w", err)
		}
	}
	return result, nil
}

// FileEntries accepts legacy arrays and current InvenioRDM entries objects.
func FileEntries(v any) []map[string]any {
	out := []map[string]any{}
	if list, ok := v.([]any); ok {
		for _, item := range list {
			if m := Object(item); m != nil {
				out = append(out, m)
			}
		}
		return out
	}
	for key, v := range Object(Object(v)["entries"]) {
		if m := Object(v); m != nil {
			if _, exists := m["key"]; !exists {
				m["key"] = key
			}
			out = append(out, m)
		}
	}
	return out
}

func FileName(m map[string]any) string {
	for _, key := range []string{"key", "filename", "name"} {
		if s := String(m[key]); s != "" {
			return s
		}
	}
	return ""
}
