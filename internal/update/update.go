package update

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	releasesURL = "https://api.github.com/repos/northlighthq/platty/releases?per_page=20"
	sumsName    = "SHA256SUMS"
	sigName     = "SHA256SUMS.sig"
	maxMeta     = 1 << 20
	maxAsset    = 256 << 20
	metaTimeout = 30 * time.Second
	fileTimeout = 15 * time.Minute
)

var publicKey = mustKey("cnmQdnLN6e2VKGYEPoyYQKHVw7rDDutVbxn3TJbxt6A=")

var (
	ErrSignature = errors.New("update: the release signature doesn't match")
	ErrChecksum  = errors.New("update: the download doesn't match the signed checksum")
	ErrMissing   = errors.New("update: the release has no file for this build")
)

type Kind uint8

const (
	Installer Kind = iota
	Portable
)

type Release struct {
	Version string
	Page    string
	assets  map[string]string
}

type ghRelease struct {
	Tag    string `json:"tag_name"`
	Draft  bool   `json:"draft"`
	Page   string `json:"html_url"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func mustKey(s string) ed25519.PublicKey {
	b, err := base64.StdEncoding.DecodeString(s)

	if err != nil || len(b) != ed25519.PublicKeySize {
		panic("update: bad public key")
	}

	return ed25519.PublicKey(b)
}

func Check(ctx context.Context, current string) (Release, bool, error) {
	body, err := fetch(ctx, releasesURL, maxMeta, current)

	if err != nil {
		return Release{}, false, err
	}

	var list []ghRelease

	if err := json.Unmarshal(body, &list); err != nil {
		return Release{}, false, err
	}

	var best Release

	for _, r := range list {
		v := strings.TrimPrefix(r.Tag, "v")

		if r.Draft || !valid(v) {
			continue
		}

		assets := map[string]string{}

		for _, a := range r.Assets {
			assets[a.Name] = a.URL
		}

		if assets[sumsName] == "" || assets[sigName] == "" {
			continue
		}

		if best.Version == "" || Newer(v, best.Version) {
			best = Release{Version: v, Page: r.Page, assets: assets}
		}
	}

	if best.Version == "" {
		return Release{}, false, nil
	}

	return best, Newer(best.Version, current), nil
}

func (r Release) assetName(k Kind) string {
	if runtime.GOOS != "windows" {
		return ""
	}

	if k != Installer {
		return "platty-" + r.Version + "-windows-" + runtime.GOARCH + ".zip"
	}

	if runtime.GOARCH == "amd64" {
		return "platty-" + r.Version + "-setup.exe"
	}

	return "platty-" + r.Version + "-" + runtime.GOARCH + "-setup.exe"
}

func (r Release) Download(ctx context.Context, k Kind, dir string, progress func(done, total int64)) (string, error) {
	name := r.assetName(k)
	url := r.assets[name]

	if url == "" {
		return "", ErrMissing
	}

	sums, err := fetch(ctx, r.assets[sumsName], maxMeta, "")

	if err != nil {
		return "", err
	}

	sig, err := fetch(ctx, r.assets[sigName], maxMeta, "")

	if err != nil {
		return "", err
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))

	if err != nil || !ed25519.Verify(publicKey, sums, raw) {
		return "", ErrSignature
	}

	want, ok := lookup(sums, name)

	if !ok {
		return "", ErrMissing
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	dst := filepath.Join(dir, name)

	if err := save(ctx, url, dst, want, progress); err != nil {
		os.Remove(dst)
		return "", err
	}

	return dst, nil
}

func lookup(sums []byte, name string) ([]byte, bool) {
	sc := bufio.NewScanner(bytes.NewReader(sums))

	for sc.Scan() {
		hash, file, ok := strings.Cut(sc.Text(), "  ")

		if !ok || strings.TrimPrefix(file, "*") != name {
			continue
		}

		b, err := hex.DecodeString(hash)

		if err != nil || len(b) != sha256.Size {
			return nil, false
		}

		return b, true
	}

	return nil, false
}

func save(ctx context.Context, url, dst string, want []byte, progress func(done, total int64)) error {
	resp, err := get(ctx, url, "", fileTimeout)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	out, err := os.Create(dst)

	if err != nil {
		return err
	}

	h := sha256.New()
	w := &counter{w: io.MultiWriter(out, h), total: resp.ContentLength, progress: progress}
	_, err = io.Copy(w, io.LimitReader(resp.Body, maxAsset))

	if cerr := out.Close(); err == nil {
		err = cerr
	}

	if err != nil {
		return err
	}

	if !bytes.Equal(h.Sum(nil), want) {
		return ErrChecksum
	}

	return nil
}

type counter struct {
	w        io.Writer
	done     int64
	total    int64
	progress func(done, total int64)
}

func (c *counter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.done += int64(n)

	if c.progress != nil {
		c.progress(c.done, c.total)
	}

	return n, err
}

func fetch(ctx context.Context, url string, limit int64, current string) ([]byte, error) {
	resp, err := get(ctx, url, current, metaTimeout)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

func get(ctx context.Context, url, current string, timeout time.Duration) (*http.Response, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("update: refusing %q", url)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if err != nil {
		cancel()
		return nil, err
	}

	req.Header.Set("User-Agent", "Platty/"+current)
	req.Header.Set("Accept", "application/vnd.github+json, application/octet-stream")
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		cancel()
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("update: %s returned %s", url, resp.Status)
	}

	resp.Body = &cancelBody{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}
