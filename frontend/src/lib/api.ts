export async function apiFetch(
  path: string,
  options?: RequestInit
): Promise<Response> {
  const token =
    typeof window !== "undefined" ? localStorage.getItem("access_token") : null;
  const headers = new Headers(options?.headers);
  if (token) headers.set("Authorization", `Bearer ${token}`);

  let res = await fetch(path, { ...options, headers });

  if (res.status === 401 && token) {
    const refreshed = await attemptRefresh();
    if (refreshed) {
      headers.set(
        "Authorization",
        `Bearer ${localStorage.getItem("access_token")}`
      );
      res = await fetch(path, { ...options, headers });
    } else {
      localStorage.removeItem("access_token");
      localStorage.removeItem("refresh_token");
      window.location.href = "/admin/login";
    }
  }

  return res;
}

async function attemptRefresh(): Promise<boolean> {
  const refreshToken = localStorage.getItem("refresh_token");
  if (!refreshToken) return false;

  try {
    const res = await fetch("/api/auth/refresh", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });

    if (!res.ok) return false;

    const data = await res.json();
    localStorage.setItem("access_token", data.access_token);
    localStorage.setItem("refresh_token", data.refresh_token);
    return true;
  } catch {
    return false;
  }
}
