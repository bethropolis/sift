(function () {
  function copyText(text, btn) {
    var done = function () {
      var original = btn.textContent;
      btn.setAttribute("data-copied", "true");
      btn.textContent = "copied";
      setTimeout(function () {
        btn.removeAttribute("data-copied");
        btn.textContent = original;
      }, 1600);
    };

    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(done);
      return;
    }

    var ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    try { document.execCommand("copy"); } catch (e) {}
    document.body.removeChild(ta);
    done();
  }

  document.addEventListener("click", function (e) {
    var btn = e.target.closest(".copy-btn");
    if (!btn) return;
    var targetSel = btn.getAttribute("data-copy-target");
    var text = targetSel
      ? document.querySelector(targetSel).textContent.trim()
      : btn.getAttribute("data-copy");
    if (text) copyText(text, btn);
  });

  // Mobile nav disclosure: <details> alone stays open, so close it when a
  // link inside is chosen, on Escape, or on a click anywhere outside it.
  var navMenu = document.querySelector(".nav-menu");
  if (navMenu) {
    navMenu.addEventListener("click", function (e) {
      if (e.target.closest("a")) navMenu.removeAttribute("open");
    });
    document.addEventListener("click", function (e) {
      if (!navMenu.contains(e.target)) navMenu.removeAttribute("open");
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape") navMenu.removeAttribute("open");
    });
  }

  // Install tabs (radio pattern): keep aria-selected in sync so screen
  // readers hear which tab is active. One-off state sync, not a framework.
  var installTabs = document.querySelectorAll('input[name="install-tabs"]');
  if (installTabs.length) {
    installTabs.forEach(function (input) {
      input.addEventListener("change", function () {
        document.querySelectorAll('.tabs__nav label[role="tab"]').forEach(function (label) {
          label.setAttribute("aria-selected", input.checked ? "false" : label.getAttribute("aria-selected"));
        });
        var active = document.querySelector('.tabs__nav label[for="' + input.id + '"]');
        if (active) active.setAttribute("aria-selected", "true");
      });
    });
  }
})();
