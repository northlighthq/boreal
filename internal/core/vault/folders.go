package vault

import (
	"slices"
	"strings"
)

const FolderSep = "/"

func CleanFolder(path string) string {
	var parts []string

	for _, p := range strings.Split(path, FolderSep) {
		if p = strings.TrimSpace(p); p != "" && p != "." && p != ".." {
			parts = append(parts, p)
		}
	}

	return strings.Join(parts, FolderSep)
}

func FolderParent(path string) string {
	i := strings.LastIndex(path, FolderSep)

	if i < 0 {
		return ""
	}

	return path[:i]
}

func FolderName(path string) string {
	return path[strings.LastIndex(path, FolderSep)+1:]
}

func InFolder(path, folder string) bool {
	return path == folder || strings.HasPrefix(path, folder+FolderSep)
}

func reparent(path, from, to string) string {
	if !InFolder(path, from) {
		return path
	}

	return CleanFolder(to + strings.TrimPrefix(path, from))
}

func (d Data) AllFolders() []string {
	seen := map[string]struct{}{}
	add := func(path string) {
		for path = CleanFolder(path); path != ""; path = FolderParent(path) {
			seen[path] = struct{}{}
		}
	}

	for _, f := range d.Folders {
		add(f)
	}

	for _, h := range d.Hosts {
		add(h.Group)
	}

	out := make([]string, 0, len(seen))

	for f := range seen {
		out = append(out, f)
	}

	slices.SortFunc(out, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	return out
}

func (v *Vault) CreateFolder(path string) error {
	path = CleanFolder(path)

	if path == "" {
		return nil
	}

	return v.update(func(d *Data) {
		if !slices.Contains(d.Folders, path) {
			d.Folders = append(d.Folders, path)
		}
	})
}

func (v *Vault) RenameFolder(from, to string) error {
	from, to = CleanFolder(from), CleanFolder(to)

	if from == "" || to == "" || from == to || InFolder(to, from) {
		return nil
	}

	return v.update(func(d *Data) {
		for i, f := range d.Folders {
			d.Folders[i] = reparent(CleanFolder(f), from, to)
		}

		if !slices.Contains(d.Folders, to) {
			d.Folders = append(d.Folders, to)
		}

		d.Folders = compactFolders(d.Folders)

		for i := range d.Hosts {
			d.Hosts[i].Group = reparent(CleanFolder(d.Hosts[i].Group), from, to)
		}
	})
}

func (v *Vault) DeleteFolder(path string) error {
	path = CleanFolder(path)

	if path == "" {
		return nil
	}

	parent := FolderParent(path)

	return v.update(func(d *Data) {
		d.Folders = slices.DeleteFunc(d.Folders, func(f string) bool { return InFolder(CleanFolder(f), path) })

		for i := range d.Hosts {
			if InFolder(CleanFolder(d.Hosts[i].Group), path) {
				d.Hosts[i].Group = parent
			}
		}
	})
}

func (v *Vault) MoveHost(id, folder string) error {
	folder = CleanFolder(folder)

	return v.update(func(d *Data) {
		for i := range d.Hosts {
			if d.Hosts[i].ID == id {
				d.Hosts[i].Group = folder
			}
		}
	})
}

func compactFolders(in []string) []string {
	out := in[:0]
	seen := map[string]struct{}{}

	for _, f := range in {
		if _, ok := seen[f]; ok || f == "" {
			continue
		}

		seen[f] = struct{}{}
		out = append(out, f)
	}

	return out
}
