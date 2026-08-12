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
})();
