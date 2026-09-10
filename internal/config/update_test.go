package config

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func TestFindMatchingAsset(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "oml_v1.1.0_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_arm64"},
		{Name: "oml_v1.1.0_darwin_amd64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_amd64"},
		{Name: "oml_v1.1.0_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux_amd64"},
		{Name: "oml_v1.1.0_windows_amd64.zip", BrowserDownloadURL: "https://example.com/windows_amd64"},
	}

	// 1. Darwin arm64
	a, err := FindMatchingAsset(assets, "darwin", "arm64")
	if err != nil {
		t.Fatalf("expected match for darwin arm64, got error: %v", err)
	}
	if a.Name != "oml_v1.1.0_darwin_arm64.tar.gz" {
		t.Errorf("expected darwin_arm64 asset, got %s", a.Name)
	}

	// 2. Windows amd64
	a, err = FindMatchingAsset(assets, "windows", "amd64")
	if err != nil {
		t.Fatalf("expected match for windows amd64, got error: %v", err)
	}
	if a.Name != "oml_v1.1.0_windows_amd64.zip" {
		t.Errorf("expected windows_amd64 asset, got %s", a.Name)
	}

	// 3. Nightly naming pattern
	nightlyAssets := []ReleaseAsset{
		{Name: "oml_nightly_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/nightly_darwin_arm64"},
		{Name: "oml_nightly_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/nightly_linux_amd64"},
		{Name: "oml_nightly_windows_amd64.zip", BrowserDownloadURL: "https://example.com/nightly_windows_amd64"},
	}
	na, err := FindMatchingAsset(nightlyAssets, "darwin", "arm64")
	if err != nil || na.Name != "oml_nightly_darwin_arm64.tar.gz" {
		t.Fatalf("expected nightly match for darwin arm64, got %v, err: %v", na, err)
	}

	// 4. Unknown arch
	_, err = FindMatchingAsset(assets, "freebsd", "riscv64")
	if err == nil {
		t.Errorf("expected error for unsupported os/arch, got nil")
	}
}

func TestExtractBinaryFromArchive_TarGz(t *testing.T) {
	// Create an in-memory tar.gz archive with a nested oml binary
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	binaryContent := "echo 'oml binary content'"
	hdr := &tar.Header{
		Name:     "oml_v1.1.0_darwin_arm64/oml",
		Mode:     0o755,
		Size:     int64(len(binaryContent)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed writing tar header: %v", err)
	}
	if _, err := tw.Write([]byte(binaryContent)); err != nil {
		t.Fatalf("failed writing tar content: %v", err)
	}
	tw.Close()
	gw.Close()

	extracted, err := ExtractBinaryFromArchive(buf.Bytes(), "darwin")
	if err != nil {
		t.Fatalf("failed extracting tar.gz: %v", err)
	}
	if string(extracted) != binaryContent {
		t.Errorf("expected extracted content %q, got %q", binaryContent, string(extracted))
	}
}

func TestExtractBinaryFromArchive_Zip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	binaryContent := "MZ Windows PE executable dummy"
	w, err := zw.Create("oml_v1.1.0_windows_amd64/oml.exe")
	if err != nil {
		t.Fatalf("failed creating zip entry: %v", err)
	}
	if _, err := w.Write([]byte(binaryContent)); err != nil {
		t.Fatalf("failed writing zip content: %v", err)
	}
	zw.Close()

	extracted, err := ExtractBinaryFromArchive(buf.Bytes(), "windows")
	if err != nil {
		t.Fatalf("failed extracting zip: %v", err)
	}
	if string(extracted) != binaryContent {
		t.Errorf("expected extracted content %q, got %q", binaryContent, string(extracted))
	}
}

func TestExtractBinaryFromArchive_NotFound(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	tw.Close()
	gw.Close()

	_, err := ExtractBinaryFromArchive(buf.Bytes(), "linux")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got: %v", err)
	}
}
