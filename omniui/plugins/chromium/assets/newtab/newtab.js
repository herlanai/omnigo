// Author: HyperparameterSI
const form = document.querySelector("form");
form.addEventListener("submit", (event) => {
  const value = form.elements.q.value.trim();
  if (!value) {
    event.preventDefault();
    return;
  }
  const isAddress =
    /^https?:\/\//i.test(value) || /^(localhost|[^\s/]+\.[^\s/]+)(:\d+)?(\/.*)?$/i.test(value);
  if (!isAddress) return;
  try {
    const target = new URL(/^https?:\/\//i.test(value) ? value : `https://${value}`);
    if (!["http:", "https:"].includes(target.protocol)) return;
    event.preventDefault();
    window.location.assign(target.href);
  } catch {
    /* Invalid addresses remain ordinary search queries. */
  }
});
const version = new URLSearchParams(location.search).get("version");
if (version && /^[\d.]+$/.test(version))
  document.querySelector("#version").textContent = `Chromium ${version}`;

const branding = window.browserBranding || {};
const appName =
  new URLSearchParams(location.search).get("appName") || branding.name || "Omni Browser";
const brand = document.querySelector(".brand");
const logo = brand.querySelector("img");
// The default logo is the OmniGo mark, not an upstream wordmark image.
const isDefault = appName === "Omni Browser";
logo.alt = appName;
logo.src = branding.icon || "logo.svg";
logo.hidden = false;
logo.classList.add("product-icon");
brand.querySelector("strong").textContent = isDefault ? "OmniGo" : appName;
brand.querySelector("strong").hidden = false;
brand.querySelector("span").hidden = !isDefault;
