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

  function sleep(ms) {
    return new Promise((r) => setTimeout(r, ms));
  }

  function safeDecode(s) {
    try { return decodeURIComponent(s); } catch (_) { return s; }
  }

  /** initData from Bridge or #WebAppData=… fragment (MAX docs). */
  function readInitData() {
    try {
      const wa = global.WebApp;
      if (wa && wa.initData) return String(wa.initData);
    } catch (_) {}
    try {
      const hash = (location.hash || '').replace(/^#/, '');
      if (!hash) return '';
      const params = new URLSearchParams(hash);
      // URLSearchParams already decodes once; optional second decode for double-encoded values.
      let raw = params.get('WebAppData') || params.get('webAppData') || '';
      if (!raw && hash.indexOf('WebAppData=') >= 0) {
        const m = hash.match(/(?:^|&)WebAppData=([^&]*)/i);
        if (m) raw = safeDecode(m[1]);
      }
      if (!raw) return '';
      // Prefer string that already contains hash= (ready for server).
      if (raw.indexOf('hash=') >= 0) return raw;
      return safeDecode(raw);
    } catch (_) {
      return '';
    }
  }

  function inMaxWebApp() {
    try {
      if (global.WebApp && (global.WebApp.initData || global.WebApp.initDataUnsafe)) return true;
      const h = location.hash || '';
      return h.indexOf('WebAppData=') >= 0 || h.indexOf('webAppData=') >= 0;
    } catch (_) {
      return false;
    }
  }

  async function waitInitData(tries) {
    for (let i = 0; i < (tries || 15); i++) {
      try {
        const wa = global.WebApp;
        if (wa && typeof wa.ready === 'function') wa.ready();
        if (wa && typeof wa.expand === 'function') wa.expand();
      } catch (_) {}
      const d = readInitData();
      if (d) return d;
      await sleep(150);
    }
    return readInitData();
  }

  /** Login via MAX Bridge initData when opened as mini-app inside MAX. */
  function ensureMaxWebAppAuth() {
    if (maxAuthPromise) return maxAuthPromise;
    maxAuthPromise = (async () => {
      try {
        if (!inMaxWebApp()) return false;
        // Already have session?
        try {
          const me = await fetch('/api/auth/me', { credentials: 'include', headers: authHeaders() });
          if (me.ok) {
            const data = await me.json();
            if (data.authenticated) return true;
          }
        } catch (_) {}

        const initData = await waitInitData(20);
        if (!initData) {
          console.warn('UVO: MAX initData empty');
          return false;
        }
        const headers = { 'Content-Type': 'application/json' };
        const csrf = csrfToken();
        if (csrf) headers['X-CSRF-Token'] = csrf;
        const res = await fetch('/api/auth/max-webapp', {
          method: 'POST',
          credentials: 'include',
          headers: headers,
          body: JSON.stringify({ init_data: initData }),
        });
        if (!res.ok) {
          const errBody = await res.text().catch(() => '');
          console.warn('UVO: max-webapp auth failed', res.status, errBody.slice(0, 200));
          maxAuthPromise = null; // allow retry
          return false;
        }
        const data = await res.json().catch(() => ({}));
        if (data.token) setToken(data.token);
        return true;
      } catch (e) {
        console.warn('UVO: max-webapp auth error', e);
        maxAuthPromise = null;
        return false;
      }
    })();
    return maxAuthPromise;
  }

  // Boot: MAX initData / ?code= / legacy ?token=
  (async function captureLoginCode() {
    try {
      await ensureMaxWebAppAuth();
      const u = new URL(location.href);
      const code = u.searchParams.get('code');
      if (code) {
        u.searchParams.delete('code');
        history.replaceState({}, '', u.pathname + u.search + u.hash);
        const headers = { 'Content-Type': 'application/json' };
        const csrf = csrfToken();
        if (csrf) headers['X-CSRF-Token'] = csrf;
        await fetch('/api/auth/exchange', {
          method: 'POST',
          credentials: 'include',
          headers: headers,
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
    const ok = await ensureMaxWebAppAuth();
    const headers = authHeaders(opts.headers);
    if (opts.body instanceof FormData) {
      delete headers['Content-Type'];
    }
    let res;
    try {
      res = await fetch(path, Object.assign({ credentials: 'include' }, opts, { headers }));
    } catch (e) {
      // One retry — MAX WebView often drops idle connections during long AceMusic waits.
      await new Promise((r) => setTimeout(r, 800));
      try {
        res = await fetch(path, Object.assign({ credentials: 'include' }, opts, { headers }));
      } catch (e2) {
        const err = new Error('Нет связи с сервером. Проверьте сеть и нажмите «Запуск» снова.');
        err.cause = e2;
        throw err;
      }
    }
    if (res.status === 401) {
      maxAuthPromise = null;
      await ensureMaxWebAppAuth();
      const headers2 = authHeaders(opts.headers);
      if (opts.body instanceof FormData) delete headers2['Content-Type'];
      let res2;
      try {
        res2 = await fetch(path, Object.assign({ credentials: 'include' }, opts, { headers: headers2 }));
      } catch (e) {
        const err = new Error('Нет связи с сервером. Проверьте сеть и нажмите «Запуск» снова.');
        err.cause = e;
        throw err;
      }
      if (res2.status !== 401) return res2;
      const err = new Error(inMaxWebApp()
        ? (ok
          ? 'Сессия MAX не принята — закройте и снова нажмите «Запуск»'
          : 'Нет initData MAX — откройте студию кнопкой «Запуск» в боте')
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
      headers: authHeaders(),
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

  function toAbsoluteURL(url) {
    if (!url) return '';
    if (/^https?:\/\//i.test(url)) return url;
    if (url.startsWith('/')) return (location.origin || '') + url;
    return url;
  }

  function trackIdFromURL(url) {
    const m = String(url || '').match(/\/tracks\/(\d+)(?:\/|$|\?)/);
    return m ? m[1] : '';
  }

  /** Resolve a signed absolute HTTPS download URL (MAX native download cannot send Bearer). */
  async function resolveDownloadURL(url, filename) {
    let abs = toAbsoluteURL(url);
    const tid = trackIdFromURL(url);
    const hasSig = /[?&]sig=/.test(abs);
    if (tid && !hasSig) {
      const res = await api('/api/tracks/' + tid + '/download-url');
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        const msg = (data.error && data.error.message) || data.error || ('HTTP ' + res.status);
        throw new Error(typeof msg === 'string' ? msg : 'Не удалось получить ссылку');
      }
      abs = toAbsoluteURL(data.url || data.path || '');
      if (data.filename && !filename) filename = data.filename;
    }
    if (!abs) throw new Error('Нет ссылки для скачивания');
    return { url: abs, filename: filename || ('uvo-track' + (tid ? '-' + tid : '') + '.mp3') };
  }

  /**
   * Download track: prefer MAX WebApp.downloadFile (native), then openLink, then blob.
   * Must be called from a user gesture (click).
   */
  async function downloadFile(url, filename) {
    if (!url) throw new Error('Нет ссылки для скачивания');
    await ensureMaxWebAppAuth();
    const resolved = await resolveDownloadURL(url, filename);
    const abs = resolved.url;
    filename = resolved.filename;

    const wa = global.WebApp;
    if (wa && typeof wa.downloadFile === 'function') {
      try {
        const ret = wa.downloadFile(abs, filename);
        if (ret && typeof ret.then === 'function') await ret;
        return true;
      } catch (e) {
        console.warn('UVO: WebApp.downloadFile failed', e);
      }
    }
    if (wa && typeof wa.openLink === 'function') {
      try {
        wa.openLink(abs);
        return true;
      } catch (e) {
        console.warn('UVO: WebApp.openLink failed', e);
      }
    }

    // Desktop / non-MAX fallback
    const headers = {};
    const tok = getToken();
    if (tok) headers['Authorization'] = 'Bearer ' + tok;
    let res;
    try {
      res = await fetch(abs, { credentials: 'include', headers });
    } catch (e) {
      throw new Error('Нет связи с сервером — не удалось скачать');
    }
    if (!res.ok) {
      let msg = 'Не удалось скачать трек (' + res.status + ')';
      try {
        const j = await res.json();
        if (j && j.error && j.error.message) msg = j.error.message;
        else if (typeof j.error === 'string') msg = j.error;
      } catch (_) {}
      throw new Error(msg);
    }
    const blob = await res.blob();
    if (!blob || blob.size < 64) throw new Error('Пустой файл трека');
    const a = document.createElement('a');
    const obj = URL.createObjectURL(blob);
    a.href = obj;
    a.download = filename || 'uvo-track.mp3';
    a.rel = 'noopener';
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(obj), 4000);
    return true;
  }

  /** Same-origin navigate keeping MAX #WebAppData hash (session depends on it). */
  function go(path) {
    try {
      const u = new URL(path, location.origin);
      if (!u.hash && location.hash) u.hash = location.hash;
      location.assign(u.pathname + u.search + u.hash);
    } catch (_) {
      location.href = path;
    }
  }

  function bindHashNav() {
    if (document.documentElement.getAttribute('data-uvo-hash-nav') === '1') return;
    document.documentElement.setAttribute('data-uvo-hash-nav', '1');
    document.addEventListener('click', (e) => {
      const a = e.target && e.target.closest && e.target.closest('a[href]');
      if (!a || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
      const href = a.getAttribute('href') || '';
      if (!href || href.charAt(0) === '#' || href.indexOf('://') >= 0 || href.indexOf('mailto:') === 0) return;
      if (href.charAt(0) !== '/') return;
      if (!location.hash) return;
      e.preventDefault();
      go(href);
    }, true);
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', bindHashNav);
  else bindHashNav();

  global.UVO = {
    getToken, setToken, csrfToken, authHeaders, api, ensureDevToken, sessionOK,
    el, mountAuthBar, ensureMaxWebAppAuth, inMaxWebApp, readInitData, downloadFile, go,
  };
})(window);
