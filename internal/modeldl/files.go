package modeldl

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// File is one artifact of a model that is distributed as individual files
// instead of a .tar.bz2 archive.
type File struct {
	Name   string
	URL    string
	Size   int64
	SHA256 string
}

// InstallFiles downloads a set of files into <root>/<id>. It keeps the same
// state files and SHA-256 checks as Install.
func InstallFiles(root, id string, files []File, required int64, force bool) error {
	if len(files) == 0 {
		return fmt.Errorf("%s no tiene archivos", id)
	}
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

	var total int64
	for _, f := range files {
		total += f.Size
	}
	st := State{State: "downloading", Total: total}
	writeState(root, id, st)

	client := &http.Client{Timeout: 30 * time.Minute}
	for _, f := range files {
		rel := filepath.Clean(f.Name)
		if rel == "." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fail(root, id, fmt.Errorf("ruta de archivo inválida: %s", f.Name))
		}
		out := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return fail(root, id, err)
		}
		req, err := http.NewRequest(http.MethodGet, f.URL, nil)
		if err != nil {
			return fail(root, id, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fail(root, id, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fail(root, id, fmt.Errorf("descarga %s: %s", f.Name, resp.Status))
		}
		tmp, err := os.CreateTemp(dest, ".file-*")
		if err != nil {
			resp.Body.Close()
			return fail(root, id, err)
		}
		tmpName := tmp.Name()
		hasher := sha256.New()
		buf := make([]byte, 1<<20)
		last := time.Now()
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				if _, werr := tmp.Write(buf[:n]); werr != nil {
					tmp.Close()
					resp.Body.Close()
					os.Remove(tmpName)
					return fail(root, id, werr)
				}
				hasher.Write(buf[:n])
				st.Downloaded += int64(n)
				if time.Since(last) > 500*time.Millisecond {
					writeState(root, id, st)
					last = time.Now()
				}
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				tmp.Close()
				resp.Body.Close()
				os.Remove(tmpName)
				return fail(root, id, rerr)
			}
		}
		resp.Body.Close()
		if err := tmp.Close(); err != nil {
			os.Remove(tmpName)
			return fail(root, id, err)
		}
		got := hex.EncodeToString(hasher.Sum(nil))
		if f.SHA256 != "" && !strings.EqualFold(got, f.SHA256) {
			os.Remove(tmpName)
			return fail(root, id, fmt.Errorf("SHA-256 de %s no coincide", f.Name))
		}
		if err := os.Rename(tmpName, out); err != nil {
			os.Remove(tmpName)
			return fail(root, id, err)
		}
	}
	writeState(root, id, State{State: "ready", Downloaded: total, Total: total})
	if err := os.WriteFile(installedPath(root, id), []byte(`{"archive":"files"}`), 0o644); err != nil {
		return fail(root, id, err)
	}
	return nil
}
