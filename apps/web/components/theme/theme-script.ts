/**
 * Constant (no interpolated values) bootstrap script, inlined into <head> so the persisted theme
 * preference applies before first paint and there is no light-to-dark flash. Reads the same
 * "sidus-theme" localStorage key that ThemeToggle writes.
 */
export const THEME_BOOTSTRAP_SCRIPT = `(function(){try{var v=window.localStorage.getItem("sidus-theme");if(v==="light"||v==="dark"){document.documentElement.setAttribute("data-theme",v);}}catch(e){}})();`;
