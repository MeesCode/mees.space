import { describe, it, expect, beforeEach } from "vitest";
import { isLoggedIn, logout } from "./auth";

// Build an unsigned JWT whose payload has the given exp (seconds since epoch).
// The signature segment is arbitrary — isLoggedIn only decodes the payload.
function makeJwt(expSeconds: number): string {
  const header = btoa(JSON.stringify({ alg: "HS256", typ: "JWT" }));
  const payload = btoa(JSON.stringify({ exp: expSeconds }));
  return `${header}.${payload}.signature`;
}

describe("isLoggedIn", () => {
  it("returns false when no token is stored", () => {
    expect(isLoggedIn()).toBe(false);
  });

  it("returns true for a token whose exp is in the future", () => {
    const future = Math.floor(new Date("2100-01-01").getTime() / 1000);
    localStorage.setItem("access_token", makeJwt(future));
    expect(isLoggedIn()).toBe(true);
  });

  it("returns false for a token whose exp is in the past", () => {
    const past = Math.floor(new Date("2000-01-01").getTime() / 1000);
    localStorage.setItem("access_token", makeJwt(past));
    expect(isLoggedIn()).toBe(false);
  });

  it("returns false for a malformed token", () => {
    localStorage.setItem("access_token", "not.a.jwt");
    expect(isLoggedIn()).toBe(false);
  });
});

describe("logout", () => {
  beforeEach(() => {
    Object.defineProperty(window, "location", {
      value: { href: "" },
      writable: true,
      configurable: true,
    });
  });

  it("clears both tokens and sets window.location.href to /", () => {
    localStorage.setItem("access_token", "a");
    localStorage.setItem("refresh_token", "b");

    logout();

    expect(localStorage.getItem("access_token")).toBeNull();
    expect(localStorage.getItem("refresh_token")).toBeNull();
    expect(window.location.href).toBe("/");
  });
});
