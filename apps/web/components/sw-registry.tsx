"use client";

import { useEffect } from "react";

export function SwRegistry() {
  useEffect(() => {
    if ("serviceWorker" in navigator) {
      window.addEventListener("load", () => {
        navigator.serviceWorker.register("/sw.js").catch((err) => {
          console.error("Service Worker registration failed:", err);
        });
      });
    }
  }, []);
  
  return null;
}
