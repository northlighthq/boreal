package vault

import "slices"

type MergeStats struct {
	Hosts      int
	Keys       int
	Forwards   int
	Snippets   int
	KnownHosts int
	Folders    int
	Conflicts  int
}

func (s MergeStats) Total() int {
	return s.Hosts + s.Keys + s.Forwards + s.Snippets + s.KnownHosts + s.Folders
}

func (v *Vault) Merge(in Data) (MergeStats, error) {
	var st MergeStats

	err := v.update(func(d *Data) {
		for _, h := range in.Hosts {
			d.Hosts = upsert(d.Hosts, h, func(x Host) string { return x.ID })
			st.Hosts++
		}

		for _, k := range in.Keys {
			d.Keys = upsert(d.Keys, k, func(x Key) string { return x.ID })
			st.Keys++
		}

		for _, f := range in.Forwards {
			d.Forwards = upsert(d.Forwards, f, func(x Forward) string { return x.ID })
			st.Forwards++
		}

		for _, s := range in.Snippets {
			d.Snippets = upsert(d.Snippets, s, func(x Snippet) string { return x.ID })
			st.Snippets++
		}

		for _, t := range in.Triggers {
			d.Triggers = upsert(d.Triggers, t, func(x Trigger) string { return x.ID })
		}

		for _, f := range in.Folders {
			if !slices.Contains(d.Folders, f) {
				d.Folders = append(d.Folders, f)
				st.Folders++
			}
		}

		for _, k := range in.KnownHosts {
			i := slices.IndexFunc(d.KnownHosts, func(x KnownHost) bool { return x.Host == k.Host })

			switch {
			case i < 0:
				d.KnownHosts = append(d.KnownHosts, k)
				st.KnownHosts++
			case d.KnownHosts[i].Key != k.Key:
				st.Conflicts++
			}
		}
	})

	return st, err
}

func (v *Vault) SavedPassphrases() int {
	n := 0

	for _, k := range v.Snapshot().Keys {
		if k.Passphrase != "" {
			n++
		}
	}

	return n
}

func (v *Vault) ForgetPassphrases() error {
	return v.update(func(d *Data) {
		for i := range d.Keys {
			d.Keys[i].Passphrase = ""
		}
	})
}
