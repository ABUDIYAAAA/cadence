import { notificationsService } from "@/services/notifications.service";

/**
 * Converts a URL-safe base64 string to a Uint8Array for applicationServerKey
 */
export function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding)
    .replace(/-/g, "+")
    .replace(/_/g, "/");

  const rawData = window.atob(base64);
  const buffer = new ArrayBuffer(rawData.length);
  const outputArray = new Uint8Array(buffer);

  for (let i = 0; i < rawData.length; ++i) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
}

/**
 * Checks whether the current browser environment supports Push Notifications & Service Workers.
 */
export function isPushSupported(): boolean {
  return (
    typeof window !== "undefined" &&
    "serviceWorker" in navigator &&
    "PushManager" in window &&
    "Notification" in window
  );
}

/**
 * Registers the Cadence Service Worker safely without hanging.
 */
export async function registerServiceWorker(): Promise<ServiceWorkerRegistration | null> {
  if (!isPushSupported()) return null;

  try {
    const registration = await navigator.serviceWorker.register("/sw.js", {
      scope: "/",
    });
    // Wait for SW ready with a 2-second timeout race
    const timeout = new Promise<null>((resolve) => setTimeout(() => resolve(null), 2000));
    await Promise.race([navigator.serviceWorker.ready, timeout]);
    return registration;
  } catch (err) {
    console.warn("Failed to register service worker:", err);
    return null;
  }
}

/**
 * Synchronizes the current browser push subscription with the backend server.
 * Prompts user for permission if permission is "default" and promptIfDefault is true.
 */
export async function syncPushSubscription(options: {
  promptIfDefault?: boolean;
} = {}): Promise<{
  success: boolean;
  status: NotificationPermission | "unsupported";
  message?: string;
}> {
  if (!isPushSupported()) {
    return { success: false, status: "unsupported", message: "Push not supported by this browser" };
  }

  let permission = Notification.permission;

  if (permission === "default" && options.promptIfDefault) {
    permission = await Notification.requestPermission();
  }

  if (permission !== "granted") {
    return {
      success: false,
      status: permission,
      message: permission === "denied" ? "Push notifications blocked by user" : "Permission not granted",
    };
  }

  const registration = await registerServiceWorker();
  if (!registration) {
    return { success: false, status: permission, message: "Service worker not active" };
  }

  try {
    // 1. Get or create browser push subscription
    let subscription = await registration.pushManager.getSubscription();

    if (!subscription) {
      // Fetch VAPID public key from backend
      const { public_key } = await notificationsService.getVAPIDPublicKey();
      if (!public_key) {
        return { success: false, status: permission, message: "No VAPID key received from server" };
      }

      subscription = await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(public_key) as BufferSource,
      });
    }

    // 2. Parse subscription credentials
    const subJson = subscription.toJSON();
    if (!subJson.endpoint || !subJson.keys?.p256dh || !subJson.keys?.auth) {
      return { success: false, status: permission, message: "Incomplete push subscription payload" };
    }

    // 3. Register/Sync subscription to backend database
    await notificationsService.subscribe({
      endpoint: subJson.endpoint,
      keys: {
        p256dh: subJson.keys.p256dh,
        auth: subJson.keys.auth,
      },
    });

    return {
      success: true,
      status: "granted",
      message: "Push notifications active",
    };
  } catch (err: any) {
    console.error("Push subscription sync failed:", err);
    return {
      success: false,
      status: permission,
      message: err?.error || err?.message || "Failed to sync push subscription",
    };
  }
}

/**
 * Unsubscribes from push notifications both in the browser and backend.
 * Uses navigator.serviceWorker.getRegistration() to avoid hanging on `.ready`.
 */
export async function unsubscribePush(): Promise<boolean> {
  if (!isPushSupported()) return false;

  try {
    // getRegistration immediately returns active registration or undefined (NEVER hangs)
    const registration = await navigator.serviceWorker.getRegistration();
    if (!registration) return true;

    const subscription = await registration.pushManager.getSubscription();
    if (subscription) {
      // 1. Tell backend to delete subscription for this user
      try {
        await notificationsService.unsubscribe({ endpoint: subscription.endpoint });
      } catch (backendErr) {
        console.warn("Backend unsubscribe error (token may already be cleared):", backendErr);
      }

      // 2. Drop push subscription locally in browser
      try {
        await subscription.unsubscribe();
      } catch (browserErr) {
        console.warn("Browser subscription unsubscribe error:", browserErr);
      }
    }
    return true;
  } catch (err) {
    console.warn("Failed to unsubscribe push:", err);
    return false;
  }
}
