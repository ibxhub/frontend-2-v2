(function () {
  "use strict";

  const STORAGE_KEY = "frontend2.jwt";

  function captureJwtFromUrl() {
    const url = new URL(window.location.href);
    const jwt = url.searchParams.get("jwt");
    if (!jwt) return;
    window.localStorage.setItem(STORAGE_KEY, jwt);
    url.searchParams.delete("jwt");
    window.history.replaceState({}, "", url.toString());
  }

  function jwt() {
    return window.localStorage.getItem(STORAGE_KEY) || "";
  }

  function handleAuthResponse(resp) {
    if (resp.status === 401) {
      window.localStorage.removeItem(STORAGE_KEY);
      window.location.href = "/auth/login";
      return false;
    }
    return true;
  }

  function wireFilePicker() {
    const input = document.getElementById("file");
    const label = document.getElementById("file-name");
    if (!input || !label) return;
    input.addEventListener("change", function () {
      label.textContent = input.files && input.files[0] ? input.files[0].name : "No file selected";
    });
  }

  function wireUpload() {
    const form = document.getElementById("upload-form");
    if (!form) return;
    form.addEventListener("submit", async function (event) {
      event.preventDefault();
      const input = document.getElementById("file");
      if (!input || !input.files || !input.files[0]) return;
      const data = new FormData();
      data.append("file", input.files[0]);
      try {
        const resp = await fetch("/upload", {
          method: "POST",
          headers: {
            Authorization: "Bearer " + jwt(),
            "X-Requested-With": "fetch",
          },
          body: data,
          redirect: "follow",
        });
        if (!handleAuthResponse(resp)) return;
        window.location.href = "/?ok=" + encodeURIComponent(input.files[0].name);
      } catch (err) {
        window.alert("Upload failed. Please try again.");
      }
    });
  }

  function wireDownloads() {
    document.querySelectorAll(".download-link").forEach(function (link) {
      link.addEventListener("click", async function (event) {
        event.preventDefault();
        const name = link.getAttribute("data-name");
        try {
          const resp = await fetch("/download/" + encodeURIComponent(name), {
            headers: {
              Authorization: "Bearer " + jwt(),
              "X-Requested-With": "fetch",
            },
          });
          if (!handleAuthResponse(resp)) return;
          if (!resp.ok) {
            window.alert("Download failed.");
            return;
          }
          const blob = await resp.blob();
          const objectUrl = window.URL.createObjectURL(blob);
          const a = document.createElement("a");
          a.href = objectUrl;
          a.download = name;
          document.body.appendChild(a);
          a.click();
          a.remove();
          window.URL.revokeObjectURL(objectUrl);
        } catch (err) {
          window.alert("Download failed.");
        }
      });
    });
  }

  function init() {
    captureJwtFromUrl();
    if (!jwt()) {
      window.location.href = "/auth/login";
      return;
    }
    wireFilePicker();
    wireUpload();
    wireDownloads();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
