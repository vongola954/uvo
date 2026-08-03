/** UVO auth — MAX Mini App + HttpOnly cookie + CSRF */
(function (global) {
  const TOKEN_KEY = 'uvo_token';
  let maxAuthPromise = null;

  function getToken() {
    try { return localStorage.getItem(TOKEN_KEY) || ''; } catch (_) { return ''; }
  }

  function setToken(t) {
    try {
      if (t) localStorage.setItem(TOKEN_KEY, t);
      else localStorage.removeItem(TOKEN_KEY);
    } catch (_) {}
  }

  function inMaxWebApp() {
    try {
      return !!(global.WebApp && (global.WebApp.initData || global.WebApp.initDataUnsafe));
    } catch (_) {
      return false;
    }
  }

  /** Login via MAX Bridge initData when opened as mini-app inside MAX. */
  function ensureMaxWebAppAuth() {
    if (maxAuthPromise) return maxAuthPromise;
    maxAuthPromise = (async () => {
      try {
        if (!inMaxWebApp()) return false;
        const wa = global.WebApp;
        try { if (typeof wa.ready === 'function') wa.ready(); } catch (_) {}
        try { if (typeof wa.expand === 'function') wa.expand(); } catch (_) {}
        const initData = wa.initData || '';
        if (!initData) return false;
        const res = await fetch('/api/auth/max-webapp', {
          method: 'POST',
          credentials: 'include',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ init_data: initData }),
        });
        if (!res.ok) return false;
        const data = await res.json().catch(() => ({}));
        if (data.token) setToken(data.token);
        return true;
      } catch (_) {
        return false;
      }
    })();
    return maxAuthPromise;
  }

  // One-time ?code= from MAX /login → HttpOnly cookie session
  (async function captureLoginCode() {
    try {
      await ensureMaxWebAppAuth();
      const u = new URL(location.href);
      const code = u.searchParams.get('code');
      if (code) {
        u.searchParams.delete('code');
        history.replaceState({}, '', u.pathname + u.search + u.hash);
        await fetch('/api/auth/exchange', {
          method: 'POST',
          credentials: 'include',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ code: code }),
        });
        setToken('');
        return;
      }
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
    await ensureMaxWebAppAuth();
    const headers = authHeaders(opts.headers);
    if (opts.body instanceof FormData) {
      delete headers['Content-Type'];
    }
    const res = await fetch(path, Object.assign({ credentials: 'include' }, opts, { headers }));
    if (res.status === 401) {
      const err = new Error(inMaxWebApp()
        ? 'Откройте студию кнопкой «Запуск» в боте MAX'
        : 'Нужна авторизация: в MAX-боте /login или «Запуск»');
      err.status = 401;
      throw err;
    }
    return res;
  }

  async function ensureDevToken() {
    await ensureMaxWebAppAuth();
    const me = await fetch('/api/auth/me', { credentials: 'include', headers: authHeaders() });
    if (me.ok) {
      const data = await me.json();
      if (data.authenticated) return 'cookie';
    }
    if (getToken()) return getToken();
    const res = await fetch('/api/auth/token', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: 'demo_user' }),
    });
    if (!res.ok) return '';
    const data = await res.json();
    if (data.token) setToken(data.token);
    return data.token || (data.session ? 'cookie' : '');
  }

  async function sessionOK() {
    try {
      await ensureMaxWebAppAuth();
      const res = await fetch('/api/auth/me', { credentials: 'include', headers: authHeaders() });
      if (!res.ok) return false;
      const data = await res.json();
      return !!data.authenticated;
    } catch (_) {
      return !!getToken();
    }
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
    const status = el('span', { text: 'сессия: …' });
    status.id = 'uvo-auth-status';
    sessionOK().then((ok) => {
      if (ok) status.textContent = inMaxWebApp() ? 'сессия: MAX' : 'сессия: ok';
      else status.textContent = 'сессия: нет — кнопка «Запуск» в MAX';
    });
    const btn = el('button', {
      type: 'button',
      className: 'border border-white/15 rounded-lg px-2 py-1 hover:border-emerald-500 hover:text-emerald-400',
      text: 'Demo token',
      onclick: async () => {
        try {
          const t = await ensureDevToken();
          status.textContent = t ? 'сессия: ok (demo)' : 'DEV_AUTH выключен — «Запуск» в MAX';
        } catch (e) {
          status.textContent = 'ошибка токена';
        }
      },
    });
    bar.appendChild(status);
    bar.appendChild(btn);
    parent.prepend(bar);
  }

  global.UVO = {
    getToken, setToken, csrfToken, authHeaders, api, ensureDevToken, sessionOK,
    el, mountAuthBar, ensureMaxWebAppAuth, inMaxWebApp,
  };
})(window);
