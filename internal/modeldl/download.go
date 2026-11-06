// Package modeldl downloads and installs ONNX model archives.
package modeldl

import (
	"archive/tar"
	"compress/bzip2"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type State struct {
	State      string    `json:"state"` // missing|downloading|ready|failed
	Downloaded int64     `json:"downloaded"`
	Total      int64     `json:"total"`
	Error      string    `json:"error,omitempty"`
	Updated    time.Time `json:"updated"`
}

type installed struct {
	Archive string `json:"archive"`
	SHA256  string `json:"sha256,omitempty"`
}

func statePath(root, id string) string     { return filepath.Join(root, id, ".state.json") }
func installedPath(root, id string) string { return filepath.Join(root, id, ".installed.json") }

// Status reads the artifact state. A finished install wins over a leftover
// progress file.
func Status(root, id string) State {
	if _, err := os.Stat(installedPath(root, id)); err == nil {
		return State{State: "ready"}
	}
	data, err := os.ReadFile(statePath(root, id))
	if err != nil {
		return State{State: "missing"}
	}
	var st State
	if json.Unmarshal(data, &st) != nil {
		return State{State: "missing"}
	}
	return st
}

// Describe is the short label shown by `models list`.
func Describe(s State) string {
	switch s.State {
	case "downloading":
		if s.Total > 0 {
			return fmt.Sprintf("downloading %d%%", s.Downloaded*100/s.Total)
		}
		return "downloading"
	case "failed":
		return "failed"
	case "ready":
		return "ready"
	default:
		return "missing"
	}
}

func writeState(root, id string, st State) {
	st.Updated = time.Now().UTC()
	_ = os.MkdirAll(filepath.Join(root, id), 0o755)
	data, _ := json.Marshal(st)
	_ = os.WriteFile(statePath(root, id), data, 0o644)
}

// FreeBytes reports the space available under root.
func FreeBytes(root string) (int64, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return 0, err
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(root, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}

// CheckSpace fails before a download that cannot fit in the models volume.
func CheckSpace(root string, required int64) error {
	if required <= 0 {
		return nil
	}
	free, err := FreeBytes(root)
	if err != nil {
		return err
	}
	if free < required {
		return fmt.Errorf("espacio insuficiente: necesitás %d MB, hay %d MB", required>>20, free>>20)
	}
	return nil
}

// Install downloads one .tar.bz2 archive, reports progress and extracts it
// under <root>/<id>. Existing installs are left untouched unless force is set.
func Install(root, id, url string, size int64, sha string, required int64, force bool) error {
	if !force {
		if Status(root, id).State == "ready" {
			return fmt.Errorf("%s ya está instalado (usá --force para reinstalar)", id)
		}
	}
	dest := filepath.Join(root, id)
	if force {
		_ = os.RemoveAll(dest)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if err := CheckSpace(root, required); err != nil {
		return fail(root, id, err)
	}
	writeState(root, id, State{State: "downloading"})

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fail(root, id, err)
	}
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fail(root, id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fail(root, id, fmt.Errorf("descarga: %s", resp.Status))
	}
	if size <= 0 {
		size = resp.ContentLength
	}

	tmp, err := os.CreateTemp(dest, ".archive-*.tar.bz2")
	if err != nil {
		return fail(root, id, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	hasher := sha256.New()
	buf := make([]byte, 1<<20)
	var downloaded int64
	last := time.Now()
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := tmp.Write(buf[:n]); err != nil {
				tmp.Close()
				return fail(root, id, err)
			}
			hasher.Write(buf[:n])
			downloaded += int64(n)
			if time.Since(last) > 500*time.Millisecond || (size > 0 && downloaded == size) {
				pct := int64(0)
				if size > 0 {
					pct = downloaded * 100 / size
				}
				fmt.Fprintf(os.Stderr, "\r%s: %d%% (%d/%d MB)", id, pct, downloaded>>20, size>>20)
				writeState(root, id, State{State: "downloading", Downloaded: downloaded, Total: size})
				last = time.Now()
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			tmp.Close()
			return fail(root, id, rerr)
		}
	}
	if err := tmp.Close(); err != nil {
		return fail(root, id, err)
	}
	fmt.Fprintln(os.Stderr)
	if sha != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(got, sha) {
			return fail(root, id, fmt.Errorf("sha256 no coincide: got %s", got))
		}
	}
	if err := extract(tmpName, dest); err != nil {
		return fail(root, id, err)
	}
	data, _ := json.Marshal(installed{Archive: url, SHA256: sha})
	if err := os.WriteFile(installedPath(root, id), data, 0o644); err != nil {
		return fail(root, id, err)
	}
	_ = os.Remove(statePath(root, id))
	writeState(root, id, State{State: "ready"})
	return nil
}

func fail(root, id string, err error) error {
	writeState(root, id, State{State: "failed", Error: err.Error()})
	return err
}

// Remove deletes an installed model and its state.
func Remove(root, id string) error {
	return os.RemoveAll(filepath.Join(root, id))
}

func extract(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	tr := tar.NewReader(bzip2.NewReader(f))
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("archivo inseguro en el tar: %s", hdr.Name)
		}
		target := filepath.Join(dest, name)
		if !strings.HasPrefix(target, dest+string(os.PathSeparator)) {
			return fmt.Errorf("ruta fuera del destino: %s", hdr.Name)
		}
		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, tr)
		cerr := out.Close()
		if err != nil {
			return err
		}
		if cerr != nil {
			return cerr
		}
	}
}
