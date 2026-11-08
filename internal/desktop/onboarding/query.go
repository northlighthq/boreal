package onboarding

func (d *Intro) Step() int { return d.step }

func (d *Intro) ClickTheme(i int) { d.themes[i].Click() }

func (d *Intro) ClickNext() { d.next.Click() }

func (d *Intro) SetPasswords(pass, confirm string) {
	d.password.SetText(pass)
	d.confirm.SetText(confirm)
}

func (d *Intro) PasswordError() string { return d.passErr }
