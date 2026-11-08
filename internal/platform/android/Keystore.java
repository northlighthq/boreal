package com.northlighthq.boreal;

import java.nio.ByteBuffer;
import java.security.KeyStore;

import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;

import android.security.keystore.KeyGenParameterSpec;
import android.security.keystore.KeyProperties;

public final class Keystore {
	private static final String PROVIDER = "AndroidKeyStore";
	private static final String ALIAS = "boreal-vault";
	private static final String CIPHER = "AES/GCM/NoPadding";
	private static final int IV = 12;
	private static final int TAG = 128;

	private static SecretKey key() throws Exception {
		KeyStore ks = KeyStore.getInstance(PROVIDER);
		ks.load(null);

		if (ks.containsAlias(ALIAS)) {
			return ((KeyStore.SecretKeyEntry) ks.getEntry(ALIAS, null)).getSecretKey();
		}

		KeyGenerator gen = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, PROVIDER);
		gen.init(new KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT | KeyProperties.PURPOSE_DECRYPT)
			.setBlockModes(KeyProperties.BLOCK_MODE_GCM)
			.setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
			.setKeySize(256)
			.build());
		return gen.generateKey();
	}

	public static byte[] seal(byte[] plain) {
		try {
			Cipher c = Cipher.getInstance(CIPHER);
			c.init(Cipher.ENCRYPT_MODE, key());
			byte[] iv = c.getIV();
			byte[] body = c.doFinal(plain);
			return ByteBuffer.allocate(iv.length + body.length).put(iv).put(body).array();
		} catch (Exception e) {
			return null;
		}
	}

	public static byte[] open(byte[] sealed) {
		try {
			if (sealed.length <= IV) {
				return null;
			}

			Cipher c = Cipher.getInstance(CIPHER);
			c.init(Cipher.DECRYPT_MODE, key(), new GCMParameterSpec(TAG, sealed, 0, IV));
			return c.doFinal(sealed, IV, sealed.length - IV);
		} catch (Exception e) {
			return null;
		}
	}
}
