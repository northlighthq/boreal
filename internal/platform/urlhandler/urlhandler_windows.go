package urlhandler

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	appName      = "Boreal"
	classesKey   = `Software\Classes\`
	capabilities = `Software\Northlight\Boreal\Capabilities`
	registered   = `Software\RegisteredApplications`
	commandPath  = `\shell\open\command`
	progPrefix   = "Boreal."
)

var schemes = []string{"ssh", "sftp"}

func command() (string, string, error) {
	exe, err := os.Executable()

	if err != nil {
		return "", "", err
	}

	exe, err = filepath.Abs(exe)

	if err != nil {
		return "", "", err
	}

	return exe, `"` + exe + `" "%1"`, nil
}

func Registered() bool {
	exe, _, err := command()

	if err != nil {
		return false
	}

	for _, s := range schemes {
		if !ours(classesKey+s+commandPath, exe) {
			return false
		}
	}

	return true
}

func Set(enabled bool) error {
	if !enabled {
		return unregister()
	}

	exe, cmd, err := command()

	if err != nil {
		return err
	}

	for _, s := range schemes {
		if err := writeClass(classesKey+s, "URL:"+strings.ToUpper(s)+" Protocol", exe, cmd, true); err != nil {
			return err
		}

		if err := writeClass(classesKey+progPrefix+s, appName+" "+strings.ToUpper(s)+" link", exe, cmd, false); err != nil {
			return err
		}
	}

	return writeCapabilities()
}

func writeClass(path, name, exe, cmd string, protocol bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)

	if err != nil {
		return err
	}

	k.SetStringValue("", name)

	if protocol {
		k.SetStringValue("URL Protocol", "")
	}

	k.Close()

	if err := setDefault(path+`\DefaultIcon`, exe+",0"); err != nil {
		return err
	}

	return setDefault(path+commandPath, cmd)
}

func writeCapabilities() error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, capabilities, registry.SET_VALUE)

	if err != nil {
		return err
	}

	k.SetStringValue("ApplicationName", appName)
	k.SetStringValue("ApplicationDescription", "SSH and SFTP client")
	k.Close()

	urls, _, err := registry.CreateKey(registry.CURRENT_USER, capabilities+`\URLAssociations`, registry.SET_VALUE)

	if err != nil {
		return err
	}

	for _, s := range schemes {
		urls.SetStringValue(s, progPrefix+s)
	}

	urls.Close()

	apps, _, err := registry.CreateKey(registry.CURRENT_USER, registered, registry.SET_VALUE)

	if err != nil {
		return err
	}

	defer apps.Close()

	return apps.SetStringValue(appName, capabilities)
}

func setDefault(path, value string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)

	if err != nil {
		return err
	}

	defer k.Close()

	return k.SetStringValue("", value)
}

func unregister() error {
	exe, _, err := command()

	if err != nil {
		return err
	}

	for _, s := range schemes {
		if ours(classesKey+s+commandPath, exe) {
			deleteTree(classesKey + s)
		}

		deleteTree(classesKey + progPrefix + s)
	}

	deleteTree(capabilities)

	if apps, err := registry.OpenKey(registry.CURRENT_USER, registered, registry.SET_VALUE); err == nil {
		apps.DeleteValue(appName)
		apps.Close()
	}

	return nil
}

func ours(path, exe string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)

	if err != nil {
		return false
	}

	defer k.Close()

	v, _, err := k.GetStringValue("")
	return err == nil && strings.Contains(strings.ToLower(v), strings.ToLower(exe))
}

func deleteTree(path string) {
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)

	if err != nil {
		return
	}

	subs, _ := k.ReadSubKeyNames(-1)
	k.Close()

	for _, s := range subs {
		deleteTree(path + `\` + s)
	}

	registry.DeleteKey(registry.CURRENT_USER, path)
}
