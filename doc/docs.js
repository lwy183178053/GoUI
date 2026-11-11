(function () {
  "use strict";
  var body = document.body;
  var menuButton = document.getElementById("menuButton");
  var sidebar = document.getElementById("sidebar");
  var navLinks = Array.prototype.slice.call(document.querySelectorAll(".side-nav a"));
  var sections = navLinks.map(function (link) {
    return document.getElementById(link.getAttribute("href").slice(1));
  }).filter(Boolean);
  var activeId = null;
  var updatePending = false;

  function setMenu(open) {
    body.classList.toggle("sidebar-open", open);
    if (menuButton) {
      menuButton.setAttribute("aria-expanded", String(open));
      menuButton.textContent = open ? "关闭" : "目录";
    }
  }

  if (menuButton) {
    menuButton.addEventListener("click", function () { setMenu(!body.classList.contains("sidebar-open")); });
  }
  navLinks.forEach(function (link) { link.addEventListener("click", function () { setMenu(false); }); });
  document.addEventListener("click", function (event) {
    if (!body.classList.contains("sidebar-open") || !sidebar || !menuButton) return;
    if (!sidebar.contains(event.target) && !menuButton.contains(event.target)) setMenu(false);
  });

  function setActive(id) {
    if (activeId === id) return;
    activeId = id;
    navLinks.forEach(function (link) {
      var active = link.getAttribute("href") === "#" + id;
      link.classList.toggle("active", active);
      if (active) link.setAttribute("aria-current", "location");
      else link.removeAttribute("aria-current");
      if (active) {
        var group = link.closest(".nav-group");
        if (group) group.open = true;
      }
    });
  }

  function updateActive() {
    updatePending = false;
    if (sections.length === 0) return;
    var mobileBar = document.querySelector(".mobile-bar");
    var readingLine = (mobileBar ? mobileBar.getBoundingClientRect().height : 0) + 48;
    var current = sections[0];
    sections.forEach(function (section) {
      if (section.getBoundingClientRect().top <= readingLine) current = section;
    });
    if (window.scrollY > 0 && window.scrollY + window.innerHeight >= document.documentElement.scrollHeight - 2) {
      current = sections[sections.length - 1];
    }
    setActive(current.id);
  }

  function queueActiveUpdate() {
    if (updatePending) return;
    updatePending = true;
    window.requestAnimationFrame(updateActive);
  }

  window.addEventListener("scroll", queueActiveUpdate, { passive: true });
  window.addEventListener("resize", queueActiveUpdate);
  queueActiveUpdate();

  function fallbackCopy(text) {
    var area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.style.position = "fixed";
    area.style.opacity = "0";
    document.body.appendChild(area);
    area.select();
    try { document.execCommand("copy"); } finally { document.body.removeChild(area); }
  }

  document.querySelectorAll("pre").forEach(function (pre) {
    var button = document.createElement("button");
    button.type = "button";
    button.className = "copy-btn";
    button.textContent = "复制";
    button.setAttribute("aria-label", "复制代码");
    button.addEventListener("click", function () {
      var code = pre.querySelector("code");
      var text = code ? code.textContent : pre.textContent;
      var done = function () { button.textContent = "已复制"; window.setTimeout(function () { button.textContent = "复制"; }, 1200); };
      if (navigator.clipboard && window.isSecureContext) {
        navigator.clipboard.writeText(text).then(done).catch(function () { fallbackCopy(text); done(); });
      } else { fallbackCopy(text); done(); }
    });
    var wrapper = document.createElement("div");
    wrapper.className = "code-wrap";
    pre.parentNode.insertBefore(wrapper, pre);
    wrapper.appendChild(pre);
    wrapper.appendChild(button);
  });
})();
