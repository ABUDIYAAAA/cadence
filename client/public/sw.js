// Cadence Web Push Service Worker
self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener("push", (event) => {
  let data = {};
  if (event.data) {
    try {
      data = event.data.json();
    } catch {
      data = { body: event.data.text() };
    }
  }

  const title = data.title || "Cadence Notification";
  const actionUrl = data.url && data.url.trim() ? data.url.trim() : undefined;

  const options = {
    body: data.body || "New update received",
    icon: data.icon || "/favicon.ico",
    badge: "/favicon.ico",
    data: {
      url: actionUrl,
      notification_id: data.notification_id,
    },
    vibrate: [100, 50, 100],
    requireInteraction: false,
  };

  // Broadcast to open browser windows so active tabs can display the live banner
  self.clients
    .matchAll({ type: "window", includeUncontrolled: true })
    .then((clientList) => {
      for (const client of clientList) {
        client.postMessage({
          type: "LIVE_PUSH_NOTIFICATION",
          payload: {
            title,
            body: options.body,
            url: actionUrl,
            notification_id: options.data.notification_id,
          },
        });
      }
    });

  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();

  const targetUrl = event.notification.data?.url;
  if (!targetUrl) return;

  event.waitUntil(
    self.clients
      .matchAll({ type: "window", includeUncontrolled: true })
      .then((clientList) => {
        for (const client of clientList) {
          if (client.url.includes(targetUrl) && "focus" in client) {
            return client.focus();
          }
        }
        if (self.clients.openWindow) {
          return self.clients.openWindow(targetUrl);
        }
      })
  );
});
