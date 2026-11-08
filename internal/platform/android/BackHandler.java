package com.northlighthq.boreal;

import android.app.Activity;
import android.os.Build;
import android.view.View;
import android.view.WindowManager;
import android.window.OnBackInvokedCallback;
import android.window.OnBackInvokedDispatcher;

public final class BackHandler {
	public static void register(final View view) {
		if (!(view.getContext() instanceof Activity)) {
			return;
		}

		final Activity activity = (Activity) view.getContext();

		view.post(new Runnable() {
			public void run() {
				activity.getWindow().addFlags(WindowManager.LayoutParams.FLAG_SECURE);

				if (Build.VERSION.SDK_INT < 33) {
					return;
				}

				activity.getOnBackInvokedDispatcher().registerOnBackInvokedCallback(OnBackInvokedDispatcher.PRIORITY_DEFAULT, new OnBackInvokedCallback() {
					public void onBackInvoked() {
						if (!handled(view)) {
							activity.moveTaskToBack(true);
						}
					}
				});
			}
		});
	}

	private static boolean handled(View view) {
		try {
			return (Boolean) view.getClass().getMethod("backPressed").invoke(view);
		} catch (Exception e) {
			return false;
		}
	}
}
