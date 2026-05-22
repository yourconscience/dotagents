const http = require('http');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

// Configuration
const PORT = process.env.PORT || 3000;
const PUBLIC = path.join(__dirname, '..', 'public');

// SSE clients
let clients = [];

function broadcast(event) {
  const payload = `data: ${event}\n\n`;
  clients.forEach(c => c.write(payload));
}

// Git state extraction
function getGitState() {
  let log = '';
  let branch = '';
  let hash = '';
  try { branch = execSync('git branch --show-current').toString().trim(); } catch {}
  try { log = execSync('git log --oneline -20').toString().trim(); } catch {}
  try { hash = execSync('git rev-parse --short HEAD').toString().trim(); } catch {}
  if (!log) log = '(no commits yet)';
  return { branch: branch || 'main', log, hash };
}

// HTTP server
const server = http.createServer((req, res) => {
  const u = new URL(req.url, `http://127.0.0.1:${PORT}`);

  if (u.pathname === '/api/git') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(getGitState()));
    return;
  }

  if (u.pathname === '/api/events') {
    res.writeHead(200, {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      'Connection': 'keep-alive',
    });
    res.write('data: connected\n\n');
    clients.push(res);
    req.on('close', () => {
      clients = clients.filter(c => c !== res);
    });
    return;
  }

  if (u.pathname === '/api/notify' && req.method === 'POST') {
    broadcast('git-update');
    res.writeHead(204);
    res.end();
    return;
  }

  // Static asset serving
  const filePath = path.join(PUBLIC, u.pathname === '/' ? 'index.html' : u.pathname);
  try {
    const data = fs.readFileSync(filePath);
    const ext = path.extname(filePath);
    const ct = {
      '.js': 'application/javascript',
      '.css': 'text/css',
      '.html': 'text/html',
    }[ext] || 'text/plain';
    res.writeHead(200, { 'Content-Type': ct });
    res.end(data);
  } catch {
    res.writeHead(404);
    res.end('Not found');
  }
});

server.listen(PORT, () => {
  console.log(`[cmux-demo] Dashboard: http://127.0.0.1:${PORT}`);
});
