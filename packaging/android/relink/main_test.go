package main

import (
	"strings"
	"testing"
)

const gogioManifest = `<manifest>
	<uses-permission android:name="android.permission.INTERNET"/>
	<application android:label="Boreal">
		<activity android:name="org.gioui.GioActivity"/>
	</application>
</manifest>`

func TestPatch(t *testing.T) {
	m, err := patch(gogioManifest)

	if err != nil {
		t.Fatal(err)
	}

	for _, p := range permissions {
		if !strings.Contains(m, `<uses-permission android:name="`+p+`"/>`) {
			t.Errorf("missing permission %s", p)
		}
	}

	svc := strings.Index(m, "com.northlighthq.boreal.SessionService")

	if svc < 0 || svc > strings.Index(m, appEnd) || svc < strings.Index(m, "<application") {
		t.Fatal("the service must sit inside <application>")
	}

	if !strings.Contains(m, `android:foregroundServiceType="specialUse"`) {
		t.Error("the service must declare its foreground type")
	}
}

func TestPatchRejectsUnknownLayout(t *testing.T) {
	if _, err := patch("<manifest/>"); err == nil {
		t.Fatal("expected an error for a manifest without the anchors")
	}
}
