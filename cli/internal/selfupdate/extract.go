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

// maxBinarySize is the upper bound for an extracted binary (200 MB).
const maxBinarySize = 200 * 1024 * 1024

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
		if header.Size > maxBinarySize {
			return fmt.Errorf("tar entry too large: %d bytes (max %d)", header.Size, maxBinarySize)
		}
		dst, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer dst.Close()
		if _, err := io.Copy(dst, io.LimitReader(tr, maxBinarySize)); err != nil {
			return fmt.Errorf("extracting binary: %w", err)
		}
		return nil
	}
	return fmt.Errorf("binary %q not found in archive", ohBinaryName)
}
