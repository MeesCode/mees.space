import { describe, it, expect, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { act, renderHook } from "@testing-library/react";
import { NavigationProvider, useNavigation } from "./navigation";

function Wrapper({ children }: { children: ReactNode }) {
  return <NavigationProvider>{children}</NavigationProvider>;
}

describe("useNavigation", () => {
  beforeEach(() => {
    window.history.replaceState(null, "", "/start");
  });

  it("path equals window.location.pathname on first render", () => {
    const { result } = renderHook(() => useNavigation(), { wrapper: Wrapper });
    expect(result.current.path).toBe("/start");
  });

  it("navigate(href) updates path and calls window.history.pushState", () => {
    const pushSpy = vi.spyOn(window.history, "pushState");
    const { result } = renderHook(() => useNavigation(), { wrapper: Wrapper });

    act(() => {
      result.current.navigate("/foo");
    });

    expect(result.current.path).toBe("/foo");
    expect(pushSpy).toHaveBeenCalledWith(null, "", "/foo");
  });

  it("a popstate event updates path to window.location.pathname", () => {
    const { result } = renderHook(() => useNavigation(), { wrapper: Wrapper });

    act(() => {
      window.history.replaceState(null, "", "/back-target");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    expect(result.current.path).toBe("/back-target");
  });
});
