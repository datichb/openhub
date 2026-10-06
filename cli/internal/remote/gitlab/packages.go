package gitlab

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
)

// Generic package names and versions accepted by the generic registry.
var (
	packageNameRe    = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	packageVersionRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)
)

// PackageFile is an uploaded generic package file.
type PackageFile struct {
	ID         int64  `json:"id"`
	FileName   string `json:"file_name"`
	Size       int64  `json:"size"`
	FileSHA256 string `json:"file_sha256"`
}

func genericPath(project, name, version, file string) (string, error) {
	if !packageNameRe.MatchString(name) || !packageVersionRe.MatchString(version) || !packageNameRe.MatchString(file) {
		return "", fmt.Errorf("gitlab: invalid generic package reference %s/%s/%s", name, version, file)
	}
	return projectRef(project) + "/packages/generic/" + url.PathEscape(name) + "/" + url.PathEscape(version) + "/" + url.PathEscape(file), nil
}

// GenericPackageURL is the download URL of a generic package file.
func (c *Client) GenericPackageURL(project, name, version, file string) (string, error) {
	p, err := genericPath(project, name, version, file)
	if err != nil {
		return "", err
	}
	return c.base + "/api/v4" + p, nil
}

// GenericPackageExists reports whether a generic package file is present.
func (c *Client) GenericPackageExists(ctx context.Context, project, name, version, file string) (bool, error) {
	p, err := genericPath(project, name, version, file)
	if err != nil {
		return false, err
	}
	resp, err := c.send(ctx, http.MethodHead, p, nil, "")
	if IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	return true, nil
}

// UploadGenericPackage uploads body as a generic package file. size must be
// the exact length of body.
func (c *Client) UploadGenericPackage(ctx context.Context, project, name, version, file string, body io.Reader, size int64) (*PackageFile, error) {
	p, err := genericPath(project, name, version, file)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(ctx, http.MethodPut, p+"?select=package_file", sized{body, size}, "application/octet-stream")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var f PackageFile
	if err := decodeJSON(resp.Body, &f); err != nil {
		return nil, fmt.Errorf("gitlab: upload %s: %w", file, err)
	}
	return &f, nil
}

// DownloadGenericPackage streams a generic package file (caller closes).
func (c *Client) DownloadGenericPackage(ctx context.Context, project, name, version, file string) (io.ReadCloser, error) {
	p, err := genericPath(project, name, version, file)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(ctx, http.MethodGet, p, nil, "")
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// sized is a reader with a known length (sets Content-Length).
type sized struct {
	io.Reader
	n int64
}

func (s sized) Len() int { return int(s.n) }
