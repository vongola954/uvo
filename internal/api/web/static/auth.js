/** UVO auth helper — Bearer JWT + CSRF for /api/* */
(function (global) {
  const TOKEN_KEY = 'uvo_token';

  function getToken() {
    try { return localStorage.getItem(TOKEN_KEY) || ''; } catch (_) { return ''; }
  }

  function setToken(t) {
    try {
      if (t) localStorage.setItem(TOKEN_KEY, t);
      else localStorage.removeItem(TOKEN_KEY);
    } catch (_) {}
  }

  // Capture ?token= from MAX /login deep link, then strip from URL
  (function captureTokenFromURL() {
    try {
      const u = new URL(location.href);
      const t = u.searchParams.get('token');
      if (t) {
        setToken(t);
        u.searchParams.delete('token');
        history.replaceState({}, '', u.pathname + u.search + u.hash);
      }
    } catch (_) {}
  })();

  function csrfToken() {
    const m = document.cookie.match(/(?:^|;\s*)uvo_csrf=([^;]+)/);
    return m ? decodeURIComponent(m[1]) : '';
  }

  function authHeaders(extra) {
    const h = Object.assign({ 'Content-Type': 'application/json' }, extra || {});
    const tok = getToken();
    if (tok) h['Authorization'] = 'Bearer ' + tok;
    const csrf = csrfToken();
    if (csrf) h['X-CSRF-Token'] = csrf;
    return h;
  }

  async function api(path, opts) {
    opts = opts || {};
    const headers = authHeaders(opts.headers);
    if (opts.body instanceof FormData) {
      delete headers['Content-Type'];
    }
    const res = await fetch(path, Object.assign({}, opts, { headers }));
    if (res.status === 401) {
      const err = new Error('Нужна авторизация: в MAX-боте отправьте /login и откройте ссылку');
      err.status = 401;
      throw err;
    }
    return res;
  }

  async function ensureDevToken() {
    if (getToken()) return getToken();
    const res = await fetch('/api/auth/token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: 'demo_user' }),
    });
    if (!res.ok) return '';
    const data = await res.json();
    if (data.token) setToken(data.token);
    return data.token || '';
  }

  function el(tag, props, children) {
    const node = document.createElement(tag);
    if (props) {
      Object.keys(props).forEach((k) => {
        if (k === 'className') node.className = props[k];
        else if (k === 'text') node.textContent = props[k];
        else if (k.startsWith('on') && typeof props[k] === 'function') node.addEventListener(k.slice(2).toLowerCase(), props[k]);
        else if (k === 'src' || k === 'href' || k === 'controls' || k === 'download') node.setAttribute(k === 'controls' ? 'controls' : k, props[k] === true ? '' : props[k]);
        else node.setAttribute(k, props[k]);
      });
    }
    (children || []).forEach((c) => {
      if (c == null) return;
      node.appendChild(typeof c === 'string' ? document.createTextNode(c) : c);
    });
    return node;
  }

  function mountAuthBar(parent) {
    if (!parent) return;
    const bar = el('div', { className: 'flex items-center gap-2 text-xs text-zinc-500 mb-4' });
    const status = el('span', { text: getToken() ? 'сессия: ok' : 'сессия: нет — MAX /login' });
    status.id = 'uvo-auth-status';
    const btn = el('button', {
      type: 'button',
      className: 'border border-white/15 rounded-lg px-2 py-1 hover:border-emerald-500 hover:text-emerald-400',
      text: 'Demo token',
      onclick: async () => {
        try {
          const t = await ensureDevToken();
          status.textContent = t ? 'сессия: ok (demo)' : 'DEV_AUTH выключен — используйте /login в MAX';
        } catch (e) {
          status.textContent = 'ошибка токена';
        }
      },
    });
    bar.appendChild(status);
    bar.appendChild(btn);
    parent.prepend(bar);
  }

  global.UVO = { getToken, setToken, csrfToken, authHeaders, api, ensureDevToken, el, mountAuthBar };
})(window);
