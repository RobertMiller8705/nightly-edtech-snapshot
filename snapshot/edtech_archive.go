package snapshot

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ArchiveDirectory(root string) ([]byte, error) {
	var output bytes.Buffer
	gz, err := gzip.NewWriterLevel(&output, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("create gzip writer: %w", err)
	}
	gz.Name = ""
	gz.Comment = ""
	gz.ModTime = time.Unix(0, 0).UTC()
	tarWriter := tar.NewWriter(gz)

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("snapshot input contains a non-regular file: %s", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		header.ModTime = time.Unix(0, 0).UTC()
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tarWriter, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return nil, fmt.Errorf("archive %s: %w", root, err)
	}
	if err := tarWriter.Close(); err != nil {
		return nil, fmt.Errorf("close tar archive: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("close gzip archive: %w", err)
	}
	return output.Bytes(), nil
}

func ObjectKey(dataset string, date time.Time) (string, error) {
	dataset = strings.TrimSpace(dataset)
	if dataset == "" || strings.ContainsAny(dataset, "/\\") {
		return "", fmt.Errorf("dataset must be one path-safe name")
	}
	return fmt.Sprintf("snapshots/%s/%s.tar.gz", dataset, date.UTC().Format("2006-01-02")), nil
}
