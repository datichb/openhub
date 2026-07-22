package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const ohBinaryName = "oh"

func extractFromTarGz(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening tar.gz: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(header.Name) != ohBinaryName {
			continue
		}
		dst, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer dst.Close()
		if _, err := io.Copy(dst, tr); err != nil {
			return fmt.Errorf("extracting binary: %w", err)
		}
		return nil
	}
	return fmt.Errorf("binary %q not found in archive", ohBinaryName)
}
