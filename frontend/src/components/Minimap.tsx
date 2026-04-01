"use client";

import { useEffect, useRef, useCallback } from "react";

const SCALE = 0.15;

export function Minimap() {
  const mapRef = useRef<HTMLDivElement>(null);
  const overlayRef = useRef<HTMLDivElement>(null);
  const viewportRef = useRef<HTMLDivElement>(null);
  const dragging = useRef(false);
  const lastClientY = useRef(0);
  const viewportHeight = useRef(0);

  const resizeHandler = useCallback(() => {
    const wrapper = document.getElementById("article-wrapper");
    const contentEl = document.getElementById("content");
    const map = mapRef.current;
    const overlay = overlayRef.current;
    const viewport = viewportRef.current;
    if (!wrapper || !map || !overlay || !viewport) return;

    const width = contentEl ? contentEl.clientWidth : wrapper.clientWidth;
    document.documentElement.style.setProperty(
      "--minimap-width",
      `${width}px`
    );
    document.documentElement.style.setProperty(
      "--minimap-scale",
      String(SCALE)
    );

    overlay.style.height = `${map.scrollHeight * SCALE}px`;
    viewportHeight.current =
      (window.innerHeight / wrapper.scrollHeight) * (map.scrollHeight * SCALE);
    viewport.style.height = `${viewportHeight.current}px`;
  }, []);

  const scrollHandler = useCallback(() => {
    const wrapper = document.getElementById("article-wrapper");
    const map = mapRef.current;
    const overlay = overlayRef.current;
    const viewport = viewportRef.current;
    if (!wrapper || !map || !overlay || !viewport) return;

    const maxScroll = wrapper.scrollHeight - window.innerHeight;
    const percentage = maxScroll > 0 ? window.scrollY / maxScroll : 0;

    const top = -0.5 * (map.clientHeight - map.clientHeight * SCALE);
    const mapHeight = map.scrollHeight * SCALE;

    let offset = 0;
    if (mapHeight > window.innerHeight) {
      offset = percentage * (mapHeight - window.innerHeight);
    }

    map.style.top = `${top - offset}px`;
    overlay.style.top = `${-offset}px`;

    const scrollHeight = Math.min(map.scrollHeight * SCALE, window.innerHeight);
    viewport.style.top = `${percentage * (scrollHeight - viewportHeight.current)}px`;
  }, []);

  const dragHandler = useCallback(
    (e: MouseEvent) => {
      if (!dragging.current) return;
      const delta = e.clientY - lastClientY.current;
      lastClientY.current = e.clientY;
      window.scrollTo(0, window.scrollY + delta * (1 / SCALE));
    },
    []
  );

  const clickHandler = useCallback((e: MouseEvent) => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    const target = e.target as HTMLElement;
    const rect = target.getBoundingClientRect();
    const offsetY = e.clientY - rect.top;
    window.scrollTo(
      0,
      (offsetY - viewport.clientHeight / 2) * (1 / SCALE)
    );
  }, []);

  useEffect(() => {
    const wrapper = document.getElementById("article-wrapper");
    if (!wrapper) return;

    const map = mapRef.current;
    const overlay = overlayRef.current;
    const viewport = viewportRef.current;
    if (!map || !overlay || !viewport) return;

    // Clone content into minimap
    const updateContent = () => {
      map.innerHTML = wrapper.innerHTML;
    };
    updateContent();

    // Reflow interval
    const interval = setInterval(() => {
      updateContent();
      resizeHandler();
      scrollHandler();
    }, 500);

    // Event listeners
    const onScroll = () => scrollHandler();
    const onResize = () => resizeHandler();
    const onMouseDown = (e: MouseEvent) => {
      e.preventDefault();
      dragging.current = true;
      lastClientY.current = e.clientY;
      viewport.classList.add("dragging");
    };
    const onMouseUp = () => {
      dragging.current = false;
      viewport.classList.remove("dragging");
    };
    const onOverlayClick = (e: MouseEvent) => clickHandler(e);

    document.addEventListener("scroll", onScroll);
    window.addEventListener("resize", onResize);
    viewport.addEventListener("mousedown", onMouseDown);
    document.addEventListener("mousemove", dragHandler);
    document.addEventListener("mouseup", onMouseUp);
    overlay.addEventListener("click", onOverlayClick);

    resizeHandler();
    scrollHandler();

    return () => {
      clearInterval(interval);
      document.removeEventListener("scroll", onScroll);
      window.removeEventListener("resize", onResize);
      viewport.removeEventListener("mousedown", onMouseDown);
      document.removeEventListener("mousemove", dragHandler);
      document.removeEventListener("mouseup", onMouseUp);
      overlay.removeEventListener("click", onOverlayClick);
    };
  }, [resizeHandler, scrollHandler, dragHandler, clickHandler]);

  return (
    <>
      <div ref={mapRef} className="minimap" />
      <div ref={overlayRef} className="minimap-overlay" />
      <div ref={viewportRef} className="minimap-viewport" />
    </>
  );
}
