//go:build android

package android

import (
	"errors"

	"gioui.org/app"
	"git.wow.st/gmp/jni"
)

const (
	backClass     = "com/northlighthq/boreal/BackHandler"
	keystoreClass = "com/northlighthq/boreal/Keystore"
)

var ErrKeystore = errors.New("android: keystore operation failed")

func RegisterBack(view uintptr) {
	if view == 0 {
		return
	}

	go call(func(env jni.Env) error {
		cls, err := class(env, backClass)

		if err != nil {
			return err
		}

		mid := jni.GetStaticMethodID(env, cls, "register", "(Landroid/view/View;)V")
		return jni.CallStaticVoidMethod(env, cls, mid, jni.Value(view))
	})
}

func Seal(plain []byte) ([]byte, error) {
	return crypt("seal", plain)
}

func Open(sealed []byte) ([]byte, error) {
	return crypt("open", sealed)
}

func crypt(method string, in []byte) ([]byte, error) {
	var out []byte

	err := call(func(env jni.Env) error {
		cls, err := class(env, keystoreClass)

		if err != nil {
			return err
		}

		mid := jni.GetStaticMethodID(env, cls, method, "([B)[B")
		res, err := jni.CallStaticObjectMethod(env, cls, mid, jni.Value(jni.NewByteArray(env, in)))

		if err != nil {
			return err
		}

		if res == 0 {
			return ErrKeystore
		}

		out = jni.GetByteArrayElements(env, jni.ByteArray(res))
		return nil
	})

	return out, err
}

func call(f func(env jni.Env) error) error {
	return jni.Do(jni.JVMFor(app.JavaVM()), f)
}

func class(env jni.Env, name string) (jni.Class, error) {
	return jni.LoadClass(env, jni.ClassLoaderFor(env, jni.Object(app.AppContext())), name)
}
