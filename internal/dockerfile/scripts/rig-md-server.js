const http = require('http');
const fs = require('fs');
const path = require('path');
const { marked } = require('marked');

const PORT = parseInt(process.env.RIG_MD_PORT || '3030', 10);
const ROOT = process.env.RIG_MD_ROOT || '/workspace';

// CSS and client JS are injected at build time by the Go generator
const CSS = RIG_INJECTED_CSS;
const CLIENT_JS = RIG_INJECTED_CLIENT_JS;

// Track SSE clients for hot-reload
const clients = new Set();

// Parse .gitignore to derive ignored directory names
function loadIgnoredDirs(root) {
  var ignored = new Set(['node_modules']);
  try {
    var lines = fs.readFileSync(path.join(root, '.gitignore'), 'utf8').split('\n');
    for (var i = 0; i < lines.length; i++) {
      var line = lines[i].trim();
      if (!line || line.startsWith('#')) continue;
      // Strip trailing slash if present
      var name = line.replace(/\/+$/, '');
      // Skip patterns with path separators (nested paths) or globs — we only match simple directory names
      if (name.includes('/') || name.includes('*')) continue;
      ignored.add(name);
    }
  } catch (e) {
    // No .gitignore — just use defaults
  }
  return ignored;
}

var ignoredDirs = loadIgnoredDirs(ROOT);

function isIgnoredDir(name) {
  return name.startsWith('.') || ignoredDirs.has(name);
}

// Notify SSE clients of a markdown file change
function notifyChange(eventType, filename) {
  fileListDirty = true;
  var payload = JSON.stringify({ file: filename, event: eventType });
  for (var res of clients) {
    res.write('data: ' + payload + '\n\n');
  }
}

// Watch a single directory (non-recursive) and set up watchers on subdirectories
function watchDir(dir) {
  try {
    var watcher = fs.watch(dir, function(eventType, filename) {
      if (!filename) return;
      var fullPath = path.join(dir, filename);
      if (filename.endsWith('.md')) {
        var rel = path.relative(ROOT, fullPath);
        notifyChange(eventType, rel);
      }
      // If a new directory appears, watch it too
      try {
        if (fs.statSync(fullPath).isDirectory() && !isIgnoredDir(filename)) {
          watchDirRecursive(fullPath);
        }
      } catch (e) {
        // Directory may have already been removed — ignore
      }
    });
    watcher.on('error', function(e) {
      console.error('Watch error:', e.message);
    });
  } catch (e) {
    console.error('Watch setup error:', e.message);
  }
}

// Recursively watch a directory tree, skipping ignored directories
function watchDirRecursive(dir) {
  watchDir(dir);
  try {
    var entries = fs.readdirSync(dir, { withFileTypes: true });
    for (var i = 0; i < entries.length; i++) {
      if (entries[i].isDirectory() && !isIgnoredDir(entries[i].name)) {
        watchDirRecursive(path.join(dir, entries[i].name));
      }
    }
  } catch (e) {
    // Directory may have been removed between readdir and watch — ignore
  }
}

function findMarkdownFiles(dir, base) {
  base = base || '';
  var results = [];
  try {
    var entries = fs.readdirSync(dir, { withFileTypes: true });
    for (var i = 0; i < entries.length; i++) {
      var entry = entries[i];
      var rel = base ? base + '/' + entry.name : entry.name;
      if (isIgnoredDir(entry.name)) continue;
      if (entry.isDirectory()) {
        results = results.concat(findMarkdownFiles(path.join(dir, entry.name), rel));
      } else if (entry.name.endsWith('.md')) {
        results.push(rel);
      }
    }
  } catch (e) {}
  return results;
}

function buildNavTree(files) {
  // Group files by directory
  var tree = { _files: [] };
  for (var i = 0; i < files.length; i++) {
    var parts = files[i].split('/');
    var node = tree;
    for (var j = 0; j < parts.length - 1; j++) {
      if (!node[parts[j]]) node[parts[j]] = { _files: [] };
      node = node[parts[j]];
    }
    node._files.push(files[i]);
  }
  return tree;
}

function renderNavNode(node, name) {
  var html = '';
  // Render files at this level
  for (var i = 0; i < node._files.length; i++) {
    var f = node._files[i];
    var label = f.split('/').pop();
    html += '<li><a href="/' + encodeURIComponent(f) + '">' + label + '</a></li>';
  }
  // Render subdirectories
  var keys = Object.keys(node).filter(function(k) { return k !== '_files'; }).sort();
  for (var j = 0; j < keys.length; j++) {
    html += '<li class="dir"><span>' + keys[j] + '/</span><ul>' + renderNavNode(node[keys[j]], keys[j]) + '</ul></li>';
  }
  return html;
}

function renderNav(files) {
  if (files.length === 0) return '';
  var tree = buildNavTree(files);
  return '<nav class="nav-sidebar" id="nav-sidebar"><h3>Files</h3>' +
    '<ul>' + renderNavNode(tree, '') + '</ul></nav>';
}

function renderPage(title, body, files) {
  var nav = renderNav(files || []);
  return '<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>' +
    title + ' - Rig Docs</title><style>' + CSS + '</style></head><body><div class="layout"><div class="content">' + body +
    '</div>' + nav + '</div><script>' + CLIENT_JS + '</script></body></html>';
}

// Cached file list — refreshed on .md file changes
var cachedFiles = findMarkdownFiles(ROOT);
cachedFiles.sort();
var fileListDirty = false;

function getFiles() {
  if (fileListDirty) {
    cachedFiles = findMarkdownFiles(ROOT);
    cachedFiles.sort();
    fileListDirty = false;
  }
  return cachedFiles;
}

var server = http.createServer(function(req, res) {
  var url;
  try {
    url = decodeURIComponent(req.url.split('?')[0]);
  } catch (e) {
    res.writeHead(400, { 'Content-Type': 'text/plain' });
    res.end('Bad Request');
    return;
  }

  // SSE endpoint for hot-reload
  if (url === '/_rig/events') {
    res.writeHead(200, { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache', 'Connection': 'keep-alive', 'Access-Control-Allow-Origin': '*' });
    res.write('data: {"status":"connected"}\n\n');
    clients.add(res);
    req.on('close', function() { clients.delete(res); });
    return;
  }

  var files = getFiles();

  // Index page: serve README.md if it exists, with file listing below
  if (url === '/' || url === '') {
    var indexHtml = '';
    // Try to serve README.md as the landing page
    var readmePath = path.join(ROOT, 'README.md');
    try {
      var readmeContent = fs.readFileSync(readmePath, 'utf8');
      indexHtml += marked.parse(readmeContent);
      indexHtml += '<hr>';
    } catch (e) {}
    // Always show file listing for navigation
    indexHtml += '<h2>All Markdown Files</h2>';
    if (files.length === 0) {
      indexHtml += '<p>No markdown files found in workspace.</p>';
    } else {
      indexHtml += '<ul class="file-list">';
      for (var i = 0; i < files.length; i++) {
        indexHtml += '<li><a href="/' + encodeURIComponent(files[i]) + '">' + files[i] + '</a></li>';
      }
      indexHtml += '</ul>';
    }
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
    res.end(renderPage('Index', indexHtml, files));
    return;
  }

  // Serve markdown file
  var filePath = path.join(ROOT, url.replace(/^\//, ''));
  if (!filePath.endsWith('.md')) filePath += '.md';

  // Security: ensure we stay within ROOT
  if (!filePath.startsWith(ROOT)) {
    res.writeHead(403); res.end('Forbidden'); return;
  }

  fs.readFile(filePath, 'utf8', function(err, content) {
    if (err) {
      res.writeHead(404, { 'Content-Type': 'text/html; charset=utf-8' });
      res.end(renderPage('Not Found', '<h1>404</h1><p>File not found: ' + url + '</p><p><a href="/">Back to index</a></p>', files));
      return;
    }
    var html;
    try {
      html = marked.parse(content);
    } catch (parseErr) {
      res.writeHead(500, { 'Content-Type': 'text/html; charset=utf-8' });
      res.end(renderPage('Error', '<h1>Render Error</h1><p>Failed to parse: ' + relPath + '</p><p><a href="/">Back to index</a></p>', files));
      return;
    }
    // Rewrite relative .md links to work as server routes
    html = html.replace(/href="([^"]*?\.md)"/g, function(match, href) {
      if (href.startsWith('http://') || href.startsWith('https://')) return match;
      // Resolve relative to current file directory
      var dir = path.dirname(url.replace(/^\//, ''));
      var resolved = dir && dir !== '.' ? dir + '/' + href : href;
      return 'href="/' + resolved + '"';
    });
    var relPath = url.replace(/^\//, '');
    var parts = relPath.split('/');
    var breadcrumb = '<div class="breadcrumb">';
    for (var bi = 0; bi < parts.length - 1; bi++) {
      breadcrumb += parts[bi] + ' / ';
    }
    breadcrumb += parts[parts.length - 1] + '</div>';
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
    res.end(renderPage(relPath, breadcrumb + html, files));
  });
});

watchDirRecursive(ROOT);
server.listen(PORT, '0.0.0.0', function() {
  console.log('Rig markdown server listening on http://0.0.0.0:' + PORT);
  console.log('Serving markdown from: ' + ROOT);
});
