package main

import (
	"archive/zip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	permAnchor = "<uses-permission android:name=\"android.permission.INTERNET\"/>"
	appEnd     = "</application>"
	linked     = "link-boreal.apk"
	patched    = "AndroidManifest.boreal.xml"
)

var permissions = []string{
	"android.permission.FOREGROUND_SERVICE",
	"android.permission.FOREGROUND_SERVICE_SPECIAL_USE",
	"android.permission.POST_NOTIFICATIONS",
}

const service = `<service android:name="com.northlighthq.boreal.SessionService"
			android:foregroundServiceType="specialUse"
			android:exported="false">
			<property android:name="android.app.PROPERTY_SPECIAL_USE_FGS_SUBTYPE"
				android:value="Keeps the SSH terminal sessions the user opened connected while they switch apps"/>
		</service>
	`

func main() {
	work := flag.String("work", "", "gogio work directory")
	aapt2 := flag.String("aapt2", "", "path to aapt2")
	androidJar := flag.String("androidjar", "", "path to android.jar")
	out := flag.String("o", "", "unsigned output APK")
	flag.Parse()

	if *work == "" || *aapt2 == "" || *androidJar == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(*work, *aapt2, *androidJar, *out); err != nil {
		fmt.Fprintln(os.Stderr, "relink:", err)
		os.Exit(1)
	}
}

func run(work, aapt2, androidJar, out string) error {
	src, err := os.ReadFile(filepath.Join(work, "AndroidManifest.xml"))

	if err != nil {
		return err
	}

	manifest, err := patch(string(src))

	if err != nil {
		return err
	}

	manifestPath := filepath.Join(work, patched)

	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		return err
	}

	link := filepath.Join(work, linked)
	cmd := exec.Command(aapt2, "link", "--manifest", manifestPath, "-I", androidJar, "-o", link, filepath.Join(work, "resources.zip"))
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aapt2 link: %w", err)
	}

	return merge(out, link, filepath.Join(work, "app.zip"))
}

func patch(m string) (string, error) {
	if !strings.Contains(m, permAnchor) || !strings.Contains(m, appEnd) {
		return "", errors.New("unexpected AndroidManifest.xml layout")
	}

	var perms strings.Builder
	perms.WriteString(permAnchor)

	for _, p := range permissions {
		perms.WriteString("\n\t<uses-permission android:name=\"" + p + "\"/>")
	}

	m = strings.Replace(m, permAnchor, perms.String(), 1)
	return strings.Replace(m, appEnd, service+appEnd, 1), nil
}

func merge(out, link, app string) error {
	lz, err := zip.OpenReader(link)

	if err != nil {
		return err
	}

	defer lz.Close()

	az, err := zip.OpenReader(app)

	if err != nil {
		return err
	}

	defer az.Close()

	f, err := os.Create(out)

	if err != nil {
		return err
	}

	w := zip.NewWriter(f)
	seen := map[string]bool{}

	for _, e := range lz.File {
		seen[e.Name] = true

		if err := copyEntry(w, e); err != nil {
			f.Close()
			return err
		}
	}

	for _, e := range az.File {
		if seen[e.Name] {
			continue
		}

		if err := copyEntry(w, e); err != nil {
			f.Close()
			return err
		}
	}

	if err := w.Close(); err != nil {
		f.Close()
		return err
	}

	return f.Close()
}

func copyEntry(w *zip.Writer, e *zip.File) error {
	r, err := e.OpenRaw()

	if err != nil {
		return err
	}

	h := e.FileHeader
	dst, err := w.CreateRaw(&h)

	if err != nil {
		return err
	}

	_, err = io.Copy(dst, r)
	return err
}
