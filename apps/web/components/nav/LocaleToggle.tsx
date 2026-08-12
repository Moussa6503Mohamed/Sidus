"use client";

import { useRouter } from "next/navigation";
import styles from "./nav.module.css";

export function LocaleToggle({ currentLocale }: { currentLocale: string }) {
  const router = useRouter();

  const toggleLocale = () => {
    const nextLocale = currentLocale === "ar" ? "en" : "ar";
    document.cookie = `sidus_locale=${nextLocale}; path=/; max-age=31536000; SameSite=Lax`;
    router.refresh();
  };

  return (
    <button
      type="button"
      className={styles.navlink}
      style={{ background: "none", border: "none", cursor: "pointer" }}
      onClick={toggleLocale}
      aria-label="Toggle language"
      title="Toggle language"
    >
      {currentLocale === "ar" ? "English" : "عربي"}
    </button>
  );
}
