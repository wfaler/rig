(function() {
  // Theme toggle
  var saved = localStorage.getItem('rig-theme') || 'light';
  document.documentElement.setAttribute('data-theme', saved);
  var btn = document.createElement('button');
  btn.className = 'theme-toggle';
  btn.textContent = saved === 'dark' ? 'Light' : 'Dark';
  btn.onclick = function() {
    var cur = document.documentElement.getAttribute('data-theme');
    var next = cur === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('rig-theme', next);
    btn.textContent = next === 'dark' ? 'Light' : 'Dark';
    // Re-render mermaid with matching theme (requires page reload for full re-render)
    if (window.mermaid) { window.location.reload(); }
  };
  document.body.appendChild(btn);

  // Nav sidebar toggle
  var nav = document.getElementById('nav-sidebar');
  if (nav) {
    var navHidden = localStorage.getItem('rig-nav-hidden') === 'true';
    if (navHidden) nav.classList.add('collapsed');
    var navBtn = document.createElement('button');
    navBtn.className = 'nav-toggle';
    navBtn.innerHTML = navHidden ? '&#9776;' : '&#10005;';
    navBtn.title = 'Toggle file nav';
    navBtn.onclick = function() {
      nav.classList.toggle('collapsed');
      var hidden = nav.classList.contains('collapsed');
      localStorage.setItem('rig-nav-hidden', hidden);
      navBtn.innerHTML = hidden ? '&#9776;' : '&#10005;';
    };
    document.body.appendChild(navBtn);
    // Highlight current page and expand its parent dirs
    var cur = decodeURIComponent(window.location.pathname);
    var links = nav.querySelectorAll('a');
    for (var i = 0; i < links.length; i++) {
      var linkHref = decodeURIComponent(links[i].getAttribute('href'));
      if (linkHref === cur) {
        links[i].classList.add('active');
      }
    }
    // Directory collapse/expand click handlers
    var dirs = nav.querySelectorAll('li.dir > span');
    for (var d = 0; d < dirs.length; d++) {
      dirs[d].addEventListener('click', (function(el) { return function() { el.parentElement.classList.toggle('collapsed'); }; })(dirs[d]));
    }
    // Collapse all directories, then expand ancestors of active page
    var allDirs = nav.querySelectorAll('li.dir');
    for (var dd = 0; dd < allDirs.length; dd++) {
      allDirs[dd].classList.add('collapsed');
    }
    var activeLink = nav.querySelector('a.active');
    if (activeLink) {
      var p = activeLink.parentElement;
      while (p && p !== nav) {
        if (p.classList && p.classList.contains('dir')) p.classList.remove('collapsed');
        p = p.parentElement;
      }
    }
  }

  // Live-reload badge
  var badge = document.createElement('div');
  badge.className = 'reload-badge';
  badge.textContent = 'live';
  document.body.appendChild(badge);
  function connect() {
    var es = new EventSource('/_rig/events');
    es.onopen = function() { badge.className = 'reload-badge'; badge.textContent = 'live'; };
    es.onmessage = function(e) {
      var data = JSON.parse(e.data);
      if (!data.file) return; // Ignore connection status messages
      var current = decodeURIComponent(window.location.pathname.slice(1));
      if (data.file === current || window.location.pathname === '/') {
        window.location.reload();
      }
    };
    es.onerror = function() { badge.className = 'reload-badge disconnected'; badge.textContent = 'reconnecting'; es.close(); setTimeout(connect, 2000); };
  }
  connect();

  // Mermaid diagram rendering
  var mermaidBlocks = document.querySelectorAll('pre code.language-mermaid');
  if (mermaidBlocks.length > 0) {
    // Replace code blocks with mermaid divs before loading the library
    for (var mi = 0; mi < mermaidBlocks.length; mi++) {
      var pre = mermaidBlocks[mi].parentElement;
      var div = document.createElement('div');
      div.className = 'mermaid';
      div.textContent = mermaidBlocks[mi].textContent;
      pre.parentNode.replaceChild(div, pre);
    }
    // Load mermaid and let it auto-render all .mermaid divs
    var theme = (localStorage.getItem('rig-theme') || 'light') === 'dark' ? 'dark' : 'default';
    var s = document.createElement('script');
    s.src = 'https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js';
    s.onload = function() { mermaid.initialize({ startOnLoad: false, theme: theme }); mermaid.run(); };
    document.head.appendChild(s);
  }
})();
