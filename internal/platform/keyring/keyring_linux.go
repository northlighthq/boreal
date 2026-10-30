package keyring

import (
	"errors"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	busName       = "org.freedesktop.secrets"
	servicePath   = "/org/freedesktop/secrets"
	serviceIface  = "org.freedesktop.Secret.Service"
	collectIface  = "org.freedesktop.Secret.Collection"
	itemIface     = "org.freedesktop.Secret.Item"
	promptIface   = "org.freedesktop.Secret.Prompt"
	promptTimeout = 2 * time.Minute
	noPath        = dbus.ObjectPath("/")
)

type secret struct {
	Session     dbus.ObjectPath
	Params      []byte
	Value       []byte
	ContentType string
}

type client struct {
	conn    *dbus.Conn
	svc     dbus.BusObject
	session dbus.ObjectPath
}

func connect() (*client, error) {
	conn, err := dbus.ConnectSessionBus()

	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}

	c := &client{conn: conn, svc: conn.Object(busName, servicePath)}

	var out dbus.Variant
	err = c.svc.Call(serviceIface+".OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&out, &c.session)

	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}

	return c, nil
}

func (c *client) close() {
	c.conn.Object(busName, c.session).Call("org.freedesktop.Secret.Session.Close", 0)
	c.conn.Close()
}

func attributes(id string) map[string]string {
	return map[string]string{"application": "boreal", "id": id}
}

func (c *client) prompt(path dbus.ObjectPath) error {
	if path == noPath || path == "" {
		return nil
	}

	rule := []dbus.MatchOption{dbus.WithMatchObjectPath(path), dbus.WithMatchInterface(promptIface), dbus.WithMatchMember("Completed")}

	if err := c.conn.AddMatchSignal(rule...); err != nil {
		return err
	}

	defer c.conn.RemoveMatchSignal(rule...)

	signals := make(chan *dbus.Signal, 4)
	c.conn.Signal(signals)
	defer c.conn.RemoveSignal(signals)

	if err := c.conn.Object(busName, path).Call(promptIface+".Prompt", 0, "").Err; err != nil {
		return err
	}

	timeout := time.After(promptTimeout)

	for {
		select {
		case s := <-signals:
			if s.Path != path || len(s.Body) < 2 {
				continue
			}

			if dismissed, _ := s.Body[0].(bool); dismissed {
				return ErrDismissed
			}

			return nil
		case <-timeout:
			return ErrDismissed
		}
	}
}

func (c *client) unlock(paths []dbus.ObjectPath) error {
	var unlocked []dbus.ObjectPath
	var prompt dbus.ObjectPath

	if err := c.svc.Call(serviceIface+".Unlock", 0, paths).Store(&unlocked, &prompt); err != nil {
		return err
	}

	return c.prompt(prompt)
}

func (c *client) collection() (dbus.ObjectPath, error) {
	var path dbus.ObjectPath

	if err := c.svc.Call(serviceIface+".ReadAlias", 0, "default").Store(&path); err != nil {
		return "", err
	}

	if path == noPath {
		return "", fmt.Errorf("%w: there is no default keyring", ErrUnsupported)
	}

	return path, c.unlock([]dbus.ObjectPath{path})
}

func (c *client) find(id string) ([]dbus.ObjectPath, error) {
	var unlocked, locked []dbus.ObjectPath

	if err := c.svc.Call(serviceIface+".SearchItems", 0, attributes(id)).Store(&unlocked, &locked); err != nil {
		return nil, err
	}

	if len(locked) > 0 {
		if err := c.unlock(locked); err != nil {
			return nil, err
		}
	}

	return append(unlocked, locked...), nil
}

func Set(id, label string, value []byte) error {
	c, err := connect()

	if err != nil {
		return err
	}

	defer c.close()

	coll, err := c.collection()

	if err != nil {
		return err
	}

	props := map[string]dbus.Variant{
		itemIface + ".Label":      dbus.MakeVariant(label),
		itemIface + ".Attributes": dbus.MakeVariant(attributes(id)),
	}

	s := secret{Session: c.session, Value: value, ContentType: "application/octet-stream"}

	var item, prompt dbus.ObjectPath

	if err := c.conn.Object(busName, coll).Call(collectIface+".CreateItem", 0, props, s, true).Store(&item, &prompt); err != nil {
		return err
	}

	return c.prompt(prompt)
}

func Get(id string) ([]byte, error) {
	c, err := connect()

	if err != nil {
		return nil, err
	}

	defer c.close()

	items, err := c.find(id)

	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, ErrNotFound
	}

	var s secret

	if err := c.conn.Object(busName, items[0]).Call(itemIface+".GetSecret", 0, c.session).Store(&s); err != nil {
		return nil, err
	}

	if len(s.Value) == 0 {
		return nil, ErrNotFound
	}

	return s.Value, nil
}

func Delete(id string) error {
	c, err := connect()

	if err != nil {
		return err
	}

	defer c.close()

	items, err := c.find(id)

	if err != nil {
		return err
	}

	var errs []error

	for _, item := range items {
		var prompt dbus.ObjectPath

		if err := c.conn.Object(busName, item).Call(itemIface+".Delete", 0).Store(&prompt); err != nil {
			errs = append(errs, err)
			continue
		}

		if err := c.prompt(prompt); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
