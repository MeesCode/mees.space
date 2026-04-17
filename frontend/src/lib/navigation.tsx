"use client";

import { createContext, useContext, useState, useEffect, useCallback, ReactNode } from "react";

interface NavigationContextType {
  path: string;
  navigate: (href: string) => void;
}

const NavigationContext = createContext<NavigationContextType>({
  path: "/",
  navigate: () => {},
});

export function NavigationProvider({ children }: { children: ReactNode }) {
  const [path, setPath] = useState(() =>
    typeof window !== "undefined" ? window.location.pathname : "/"
  );

  const navigate = useCallback((href: string) => {
    window.history.pushState(null, "", href);
    setPath(href);
  }, []);

  useEffect(() => {
    const onPopState = () => setPath(window.location.pathname);
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  return (
    <NavigationContext.Provider value={{ path, navigate }}>
      {children}
    </NavigationContext.Provider>
  );
}

export function useNavigation() {
  return useContext(NavigationContext);
}
