package sshconfig

import (
	"bufio"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const (
	maxIncludeDepth = 16
	defaultPort     = 22
	none            = "none"
)

const (
	keyHost           = "host"
	keyMatch          = "match"
	keyInclude        = "include"
	keyHostName       = "hostname"
	keyUser           = "user"
	keyPort           = "port"
	keyIdentityFile   = "identityfile"
	keyCertificate    = "certificatefile"
	keyIdentitiesOnly = "identitiesonly"
	keyForwardAgent   = "forwardagent"
	keyProxyJump      = "proxyjump"
	keyProxyCommand   = "proxycommand"
)

var supported = map[string]struct{}{
	keyHostName:       {},
	keyUser:           {},
	keyPort:           {},
	keyIdentityFile:   {},
	keyCertificate:    {},
	keyIdentitiesOnly: {},
	keyForwardAgent:   {},
	keyProxyJump:      {},
	keyProxyCommand:   {},
}

var directiveNames = map[string]string{
	"addkeystoagent":           "AddKeysToAgent",
	"canonicalizehostname":     "CanonicalizeHostname",
	"compression":              "Compression",
	"controlmaster":            "ControlMaster",
	"controlpath":              "ControlPath",
	"controlpersist":           "ControlPersist",
	"dynamicforward":           "DynamicForward",
	"loglevel":                 "LogLevel",
	"localforward":             "LocalForward",
	"pubkeyacceptedkeytypes":   "PubkeyAcceptedKeyTypes",
	"pubkeyacceptedalgorithms": "PubkeyAcceptedAlgorithms",
	"remoteforward":            "RemoteForward",
	"requesttty":               "RequestTTY",
	"sendenv":                  "SendEnv",
	"serveralivecountmax":      "ServerAliveCountMax",
	"serveraliveinterval":      "ServerAliveInterval",
	"setenv":                   "SetEnv",
	"stricthostkeychecking":    "StrictHostKeyChecking",
	"usekeychain":              "UseKeychain",
	"userknownhostsfile":       "UserKnownHostsFile",
}

type Entry struct {
	Alias          string
	HostName       string
	User           string
	Port           int
	Identity       string
	Certificate    string
	IdentitiesOnly bool
	ForwardAgent   bool
	ProxyCommand   string
	Jump           []string
}

type Skipped struct {
	Name  string
	Count int
}

type Result struct {
	Entries []Entry
	Skipped []Skipped
}

type state struct {
	alias string
	entry Entry
	set   map[string]bool
}

type cond func(s *state) bool

type rule struct {
	conds []cond
	key   string
	args  []string
}

type parser struct {
	home    string
	rules   []rule
	aliases []string
	skipped map[string]int
	seen    map[string]bool
}

func Parse(path string) (Result, error) {
	home, _ := os.UserHomeDir()
	p := &parser{home: home, skipped: map[string]int{}, seen: map[string]bool{}}

	if err := p.file(path, nil, 0); err != nil {
		return Result{}, err
	}

	res := Result{}

	for _, a := range p.aliases {
		res.Entries = append(res.Entries, p.resolve(a))
	}

	for name, n := range p.skipped {
		res.Skipped = append(res.Skipped, Skipped{Name: name, Count: n})
	}

	sort.Slice(res.Skipped, func(i, j int) bool { return res.Skipped[i].Name < res.Skipped[j].Name })
	return res, nil
}

func (p *parser) file(path string, outer []cond, depth int) error {
	f, err := os.Open(path)

	if err != nil {
		return err
	}

	defer f.Close()

	cur := outer
	sc := bufio.NewScanner(f)

	for sc.Scan() {
		key, args := split(sc.Text())

		if key == "" {
			continue
		}

		switch key {
		case keyHost:
			cur = append(slices.Clip(outer), p.hostCond(args))
		case keyMatch:
			cur = append(slices.Clip(outer), p.matchCond(args))
		case keyInclude:
			p.include(args, cur, depth)
		default:
			p.directive(key, args, cur)
		}
	}

	return sc.Err()
}

func (p *parser) directive(key string, args []string, conds []cond) {
	if len(args) == 0 {
		return
	}

	if _, ok := supported[key]; !ok {
		p.skip(name(key))
		return
	}

	p.rules = append(p.rules, rule{conds: conds, key: key, args: args})
}

func (p *parser) include(args []string, conds []cond, depth int) {
	if depth >= maxIncludeDepth {
		p.skip("Include (nested too deep)")
		return
	}

	for _, pattern := range args {
		pattern = ExpandHome(pattern)

		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(p.home, ".ssh", pattern)
		}

		matches, err := filepath.Glob(pattern)

		if err != nil {
			p.skip("Include " + pattern)
			continue
		}

		sort.Strings(matches)

		for _, m := range matches {
			if err := p.file(m, conds, depth+1); err != nil {
				p.skip("Include " + m)
			}
		}
	}
}

func (p *parser) hostCond(patterns []string) cond {
	for _, pat := range patterns {
		if strings.ContainsAny(pat, "*?!") || p.seen[pat] {
			continue
		}

		p.seen[pat] = true
		p.aliases = append(p.aliases, pat)
	}

	return func(s *state) bool { return matchList(s.alias, patterns) }
}

func (p *parser) matchCond(args []string) cond {
	var conds []cond

	for i := 0; i < len(args); i++ {
		crit := strings.ToLower(args[i])
		negate := strings.HasPrefix(crit, "!")
		crit = strings.TrimPrefix(crit, "!")

		var c cond

		switch crit {
		case "all":
			c = func(*state) bool { return true }
		case "final":
			c = func(*state) bool { return true }
		case "canonical":
			c = func(*state) bool { return false }
		case "host", "originalhost", "user", "localuser":
			if i+1 >= len(args) {
				p.skip("Match " + crit)
				return func(*state) bool { return false }
			}

			i++
			c = criterion(crit, strings.Split(args[i], ","))
		default:
			p.skip("Match " + crit)

			if i+1 < len(args) {
				i++
			}

			c = func(*state) bool { return false }
		}

		if negate {
			inner := c
			c = func(s *state) bool { return !inner(s) }
		}

		conds = append(conds, c)
	}

	return func(s *state) bool { return all(conds, s) }
}

func criterion(kind string, patterns []string) cond {
	return func(s *state) bool {
		switch kind {
		case "host":
			host := s.entry.HostName

			if host == "" {
				host = s.alias
			}

			return matchList(host, patterns)
		case "originalhost":
			return matchList(s.alias, patterns)
		case "user":
			return matchList(s.entry.User, patterns)
		default:
			return matchList(localUser(), patterns)
		}
	}
}

func (p *parser) skip(name string) {
	p.skipped[name]++
}

func (p *parser) resolve(alias string) Entry {
	s := &state{alias: alias, entry: Entry{Alias: alias}, set: map[string]bool{}}

	for _, r := range p.rules {
		if !all(r.conds, s) {
			continue
		}

		s.apply(r.key, r.args)
	}

	e := s.entry

	if e.Port == 0 {
		e.Port = defaultPort
	}

	host := e.HostName

	if host == "" {
		host = alias
	}

	e.HostName = expand(e.HostName, map[byte]string{'h': alias, 'n': alias})
	vars := tokens(alias, host, e.User, e.Port)
	e.Identity = ExpandHome(expand(e.Identity, vars))
	e.Certificate = ExpandHome(expand(e.Certificate, vars))
	return e
}

func (s *state) apply(key string, args []string) {
	slot := key

	if key == keyProxyCommand {
		slot = keyProxyJump
	}

	if s.set[slot] {
		return
	}

	s.set[slot] = true
	val := args[0]

	switch key {
	case keyHostName:
		s.entry.HostName = val
	case keyUser:
		s.entry.User = val
	case keyPort:
		n, err := strconv.Atoi(val)

		if err != nil || n <= 0 || n > 65535 {
			s.set[slot] = false
			return
		}

		s.entry.Port = n
	case keyIdentityFile:
		if !strings.EqualFold(val, none) {
			s.entry.Identity = val
		}
	case keyCertificate:
		if !strings.EqualFold(val, none) {
			s.entry.Certificate = val
		}
	case keyIdentitiesOnly:
		s.entry.IdentitiesOnly = yes(val)
	case keyForwardAgent:
		s.entry.ForwardAgent = yes(val)
	case keyProxyJump:
		if strings.EqualFold(val, none) {
			return
		}

		for hop := range strings.SplitSeq(val, ",") {
			if hop = strings.TrimSpace(hop); hop != "" {
				s.entry.Jump = append(s.entry.Jump, hop)
			}
		}
	case keyProxyCommand:
		if strings.EqualFold(val, none) {
			return
		}

		s.entry.ProxyCommand = strings.Join(args, " ")
	}
}

func all(conds []cond, s *state) bool {
	for _, c := range conds {
		if !c(s) {
			return false
		}
	}

	return true
}

func split(line string) (string, []string) {
	line = strings.TrimSpace(line)

	if line == "" || strings.HasPrefix(line, "#") {
		return "", nil
	}

	end := strings.IndexAny(line, " \t=")

	if end < 0 {
		return strings.ToLower(line), nil
	}

	key := strings.ToLower(line[:end])
	rest := strings.TrimLeft(line[end:], " \t")
	rest = strings.TrimLeft(strings.TrimPrefix(rest, "="), " \t")
	return key, fields(rest)
}

func fields(s string) []string {
	var out []string
	var b strings.Builder
	quoted, inWord := false, false

	for _, r := range s {
		switch {
		case r == '"':
			quoted = !quoted
			inWord = true
		case !quoted && (r == ' ' || r == '\t'):
			if inWord {
				out = append(out, b.String())
				b.Reset()
				inWord = false
			}
		case !quoted && r == '#' && !inWord:
			return out
		default:
			b.WriteRune(r)
			inWord = true
		}
	}

	if inWord {
		out = append(out, b.String())
	}

	return out
}

func matchList(value string, patterns []string) bool {
	value = strings.ToLower(value)
	matched := false

	for _, pat := range patterns {
		pat = strings.ToLower(pat)
		neg := strings.HasPrefix(pat, "!")
		pat = strings.TrimPrefix(pat, "!")

		if !glob(pat, value) {
			continue
		}

		if neg {
			return false
		}

		matched = true
	}

	return matched
}

func glob(pattern, s string) bool {
	p, i := 0, 0
	star, mark := -1, 0

	for i < len(s) {
		switch {
		case p < len(pattern) && (pattern[p] == '?' || pattern[p] == s[i]):
			p++
			i++
		case p < len(pattern) && pattern[p] == '*':
			star, mark = p, i
			p++
		case star >= 0:
			p = star + 1
			mark++
			i = mark
		default:
			return false
		}
	}

	for p < len(pattern) && pattern[p] == '*' {
		p++
	}

	return p == len(pattern)
}

func tokens(alias, host, remoteUser string, port int) map[byte]string {
	home, _ := os.UserHomeDir()
	local, _ := os.Hostname()

	return map[byte]string{
		'd': home,
		'h': host,
		'l': local,
		'n': alias,
		'p': strconv.Itoa(port),
		'r': remoteUser,
		'u': localUser(),
	}
}

func expand(s string, vars map[byte]string) string {
	if s == "" {
		return s
	}

	var b strings.Builder

	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}

		i++

		if s[i] == '%' {
			b.WriteByte('%')
			continue
		}

		if v, ok := vars[s[i]]; ok {
			b.WriteString(v)
			continue
		}

		b.WriteByte('%')
		b.WriteByte(s[i])
	}

	return b.String()
}

func localUser() string {
	u, err := user.Current()

	if err != nil {
		return ""
	}

	_, name, ok := strings.Cut(u.Username, `\`)

	if ok {
		return name
	}

	return u.Username
}

func yes(v string) bool {
	return strings.EqualFold(v, "yes") || strings.EqualFold(v, "true")
}

func name(key string) string {
	if n, ok := directiveNames[key]; ok {
		return n
	}

	return key
}

func ExpandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[1:])
	}

	return p
}

func Format(entries []Entry) string {
	var b strings.Builder

	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n")
		}

		b.WriteString("Host " + word(e.Alias) + "\n")
		b.WriteString("    HostName " + word(e.HostName) + "\n")

		if e.User != "" {
			b.WriteString("    User " + word(e.User) + "\n")
		}

		if e.Port != 0 && e.Port != defaultPort {
			b.WriteString("    Port " + strconv.Itoa(e.Port) + "\n")
		}

		if e.Identity != "" {
			b.WriteString("    IdentityFile " + quoted(e.Identity) + "\n")
		}

		if e.IdentitiesOnly {
			b.WriteString("    IdentitiesOnly yes\n")
		}

		if e.ForwardAgent {
			b.WriteString("    ForwardAgent yes\n")
		}

		if len(e.Jump) > 0 {
			hops := make([]string, len(e.Jump))

			for i, j := range e.Jump {
				hops[i] = word(j)
			}

			b.WriteString("    ProxyJump " + strings.Join(hops, ",") + "\n")
		}

		if e.ProxyCommand != "" && len(e.Jump) == 0 {
			b.WriteString("    ProxyCommand " + line(e.ProxyCommand) + "\n")
		}
	}

	return b.String()
}

func Alias(name string) string {
	f := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-')
	})

	if len(f) == 0 {
		return "host"
	}

	return strings.Join(f, "-")
}

func word(s string) string {
	return strings.Map(func(r rune) rune {
		if r <= ' ' || r == 0x7f || r == '"' || r == '\'' || r == '#' || r == ',' {
			return -1
		}

		return r
	}, s)
}

func line(s string) string {
	return strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return -1
		}

		return r
	}, s)
}

func quoted(s string) string {
	return `"` + strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f || r == '"' {
			return -1
		}

		return r
	}, s) + `"`
}
