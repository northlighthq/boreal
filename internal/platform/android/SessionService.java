package com.northlighthq.boreal;

import android.Manifest;
import android.app.Activity;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.content.pm.ServiceInfo;
import android.os.Build;
import android.os.IBinder;
import android.view.View;

public final class SessionService extends Service {
	private static final String CHANNEL = "sessions";
	private static final String EXTRA_TITLE = "title";
	private static final String EXTRA_TEXT = "text";
	private static final int NOTIFICATION = 1;

	public static void start(Context ctx, String title, String text) {
		Intent i = new Intent(ctx, SessionService.class);
		i.putExtra(EXTRA_TITLE, title);
		i.putExtra(EXTRA_TEXT, text);

		try {
			if (Build.VERSION.SDK_INT >= 26) {
				ctx.startForegroundService(i);
			} else {
				ctx.startService(i);
			}
		} catch (RuntimeException e) {
		}
	}

	public static void stop(Context ctx) {
		ctx.stopService(new Intent(ctx, SessionService.class));
	}

	public static void askNotifications(final View view) {
		if (Build.VERSION.SDK_INT < 33 || !(view.getContext() instanceof Activity)) {
			return;
		}

		final Activity activity = (Activity) view.getContext();

		if (activity.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED) {
			return;
		}

		view.post(new Runnable() {
			public void run() {
				activity.requestPermissions(new String[] {Manifest.permission.POST_NOTIFICATIONS}, NOTIFICATION);
			}
		});
	}

	@Override public int onStartCommand(Intent intent, int flags, int startId) {
		String title = "Boreal";
		String text = "";

		if (intent != null) {
			if (intent.getStringExtra(EXTRA_TITLE) != null) {
				title = intent.getStringExtra(EXTRA_TITLE);
			}

			if (intent.getStringExtra(EXTRA_TEXT) != null) {
				text = intent.getStringExtra(EXTRA_TEXT);
			}
		}

		Notification n = notification(title, text);

		if (Build.VERSION.SDK_INT >= 34) {
			startForeground(NOTIFICATION, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE);
		} else {
			startForeground(NOTIFICATION, n);
		}

		return START_NOT_STICKY;
	}

	@Override public void onTaskRemoved(Intent root) {
		stopSelf();
	}

	@Override public IBinder onBind(Intent intent) {
		return null;
	}

	private Notification notification(String title, String text) {
		Notification.Builder b;

		if (Build.VERSION.SDK_INT >= 26) {
			NotificationManager nm = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
			NotificationChannel ch = new NotificationChannel(CHANNEL, "Open sessions", NotificationManager.IMPORTANCE_LOW);
			ch.setShowBadge(false);
			nm.createNotificationChannel(ch);
			b = new Notification.Builder(this, CHANNEL);
		} else {
			b = new Notification.Builder(this);
		}

		b.setContentTitle(title).setContentText(text).setSmallIcon(getApplicationInfo().icon).setOngoing(true).setShowWhen(false);
		Intent open = getPackageManager().getLaunchIntentForPackage(getPackageName());

		if (open != null) {
			open.setFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_RESET_TASK_IF_NEEDED);
			b.setContentIntent(PendingIntent.getActivity(this, 0, open, PendingIntent.FLAG_IMMUTABLE | PendingIntent.FLAG_UPDATE_CURRENT));
		}

		if (Build.VERSION.SDK_INT >= 31) {
			b.setForegroundServiceBehavior(Notification.FOREGROUND_SERVICE_IMMEDIATE);
		}

		return b.build();
	}
}
