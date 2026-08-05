/** UVO app shell — Sonata-like IA (bottom nav + views), UVO visual language */
(function (global) {
  const VIEWS = ['hub', 'lyrics', 'create', 'tracks', 'balance'];

  function qs(sel, root) {
    return (root || document).querySelector(sel);
  }

  function qsa(sel, root) {
    return Array.from((root || document).querySelectorAll(sel));
  }

  function hasView(name) {
    return VIEWS.indexOf(name) !== -1;
  }

  function setHiddenAttr(el, hide) {
    try {
      if (hide) el.setAttribute('hidden', '');
      else el.removeAttribute('hidden');
    } catch (_) {}
  }

  /** Same-origin jump; keep #WebAppData so MAX session survives page changes. */
  function goLocal(path) {
    try {
      const u = new URL(path, location.origin);
      if (!u.hash && location.hash) u.hash = location.hash;
      const next = u.pathname + u.search + u.hash;
      location.assign(next);
    } catch (_) {
      try { location.href = path; } catch (__) {}
    }
  }

  function setView(name, opts) {
    const view = hasView(name) ? name : 'hub';
    try { document.body.setAttribute('data-uvo-view', view); } catch (_) {}
    qsa('[data-uvo-view]').forEach((el) => {
      const on = el.getAttribute('data-uvo-view') === view;
      try { el.classList.toggle('hidden', !on); } catch (_) {}
      setHiddenAttr(el, !on);
      try { el.setAttribute('aria-hidden', on ? 'false' : 'true'); } catch (_) {}
      // Inline !important — beats stylesheet fights in MAX WebView
      try { el.style.setProperty('display', on ? 'block' : 'none', 'important'); } catch (_) {
        try { el.style.display = on ? 'block' : 'none'; } catch (__) {}
      }
    });
    qsa('[data-uvo-nav]').forEach((el) => {
      const on = el.getAttribute('data-uvo-nav') === view;
      try { el.classList.toggle('uvo-nav-active', on); } catch (_) {}
      try { el.setAttribute('aria-current', on ? 'page' : 'false'); } catch (_) {}
    });
    if (!opts || opts.scroll !== false) {
      try { window.scrollTo(0, 0); } catch (_) {}
    }
    try {
      const u = new URL(location.href);
      if (view === 'hub') u.searchParams.delete('view');
      else u.searchParams.set('view', view);
      history.replaceState({ uvoView: view }, '', u.pathname + u.search + u.hash);
    } catch (_) {}
    if (view === 'create' && opts && opts.tab) {
      const tab = document.querySelector('.studio-tab[data-tab="' + opts.tab + '"]');
      if (tab) {
        try { tab.click(); } catch (_) {}
      }
    }
    if (view === 'create' && opts && opts.mode && typeof global.setGenMode === 'function') {
      try { global.setGenMode(opts.mode); } catch (_) {}
    }
    if (view === 'tracks' && typeof global.loadTracksView === 'function') {
      try { global.loadTracksView(); } catch (_) {}
    }
  }

  function openTarget(raw) {
    const parts = String(raw || 'hub').split(':');
    const view = parts[0];
    const extra = parts[1] || '';
    if (view === 'tracks') { setView('tracks'); return; }
    if (view === 'media') {
      goLocal('/media.html' + (extra ? ('?tab=' + encodeURIComponent(extra)) : ''));
      return;
    }
    if (view === 'distribution') { goLocal('/distribution.html'); return; }
    if (view === 'playlists') { goLocal('/playlists.html'); return; }
    if (view === 'feed') { goLocal('/feed.html'); return; }
    if (view === 'karaoke') {
      goLocal('/karaoke.html' + (extra ? ('?id=' + encodeURIComponent(extra)) : ''));
      return;
    }
    const opts = {};
    if (extra === 'voice' || extra === 'cover' || extra === 'gen') opts.tab = extra === 'gen' ? 'gen' : extra;
    if (extra === 'idea' || extra === 'lyrics' || extra === 'instrumental') opts.mode = extra;
    setView(view, opts);
  }

  /** MAX WebView often targets a Text node inside the button — no .closest there. */
  function eventEl(e) {
    let t = e && e.target;
    if (!t) return null;
    if (t.nodeType === 3) t = t.parentElement || t.parentNode;
    while (t && t.nodeType !== 1) t = t.parentElement || t.parentNode;
    return t || null;
  }

  function closestAttr(start, attr) {
    let el = start;
    while (el && el.nodeType === 1) {
      if (el.getAttribute && el.getAttribute(attr) != null) return el;
      el = el.parentElement || el.parentNode;
    }
    return null;
  }

  function handleNavEvent(e) {
    if (!e) return;
    const t = eventEl(e);
    if (!t) return;
    const openEl = (typeof t.closest === 'function' ? t.closest('[data-uvo-open]') : null) || closestAttr(t, 'data-uvo-open');
    if (openEl) {
      e.preventDefault();
      e.stopPropagation();
      openTarget(openEl.getAttribute('data-uvo-open') || 'hub');
      return;
    }
    const navEl = (typeof t.closest === 'function' ? t.closest('[data-uvo-nav]') : null) || closestAttr(t, 'data-uvo-nav');
    if (!navEl) return;
    const view = navEl.getAttribute('data-uvo-nav');
    if (!view) return;
    e.preventDefault();
    e.stopPropagation();
    openTarget(view);
  }

  function bindNav() {
    if (document.documentElement.getAttribute('data-uvo-nav-bound') === '1') return;
    document.documentElement.setAttribute('data-uvo-nav-bound', '1');
    // Capture phase: MAX WebView / bridge often swallows bubble-phase clicks
    document.addEventListener('click', handleNavEvent, true);
    document.addEventListener('click', handleNavEvent, false);
    // Direct bindings as belt-and-suspenders
    qsa('[data-uvo-nav], [data-uvo-open]').forEach((el) => {
      el.addEventListener('click', handleNavEvent, false);
    });
  }

  function detectMiniApp() {
    try {
      if (global.UVO && typeof global.UVO.inMaxWebApp === 'function' && global.UVO.inMaxWebApp()) return true;
    } catch (_) {}
    const q = location.search || '';
    try {
      if (global.WebApp && (global.WebApp.initData || global.WebApp.initDataUnsafe)) return true;
    } catch (_) {}
    try {
      const h = location.hash || '';
      if (h.indexOf('WebAppData') >= 0 || h.indexOf('webAppData') >= 0) return true;
    } catch (_) {}
    return q.indexOf('app=1') >= 0 || q.indexOf('tgWebApp') >= 0;
  }

  function applyMiniAppLayout() {
    document.body.classList.toggle('uvo-miniapp', detectMiniApp());
  }

  function initFromQuery() {
    try {
      const u = new URL(location.href);
      const v = u.searchParams.get('view');
      if (v && hasView(v)) {
        setView(v, { scroll: false, smooth: false });
        return;
      }
      if (document.body.classList.contains('uvo-miniapp')) {
        setView('hub', { scroll: false, smooth: false });
      }
    } catch (_) {}
  }

  /** Style chip builder used by create/lyrics forms */
  const STYLE_GROUPS = {
    genres: [
      'Хип-хоп и R&B', 'Поп-музыка', 'Шансон', 'Джаз и блюз', 'Рок и метал',
      'Регги', 'Кантри', 'Фолк', 'Электронная музыка', 'Классика', 'Детские',
    ],
    moods: [
      'Энергичное', 'Счастливое', 'Грустное', 'Романтическое', 'Чилл',
      'Эпичное', 'Драматичное', 'Тёмное', 'Мечтательное', 'Вдохновляющее',
    ],
    voice: ['Мужской вокал', 'Женский вокал', 'Дуэт', 'Детский вокал'],
  };

  function selectedChips(root) {
    return qsa('.style-chip.is-on', root).map((b) => b.getAttribute('data-style') || b.textContent.trim());
  }

  function syncStyleField(root) {
    const input = qs('#style-custom', root) || qs('#style', root);
    if (!input) return;
    const parts = selectedChips(root);
    const custom = (qs('#style-free') && qs('#style-free').value || '').trim();
    if (custom) parts.push(custom);
    if (input.tagName === 'SELECT') {
      const val = parts.join(', ');
      let found = false;
      for (let i = 0; i < input.options.length; i++) {
        if (input.options[i].value === val) { found = true; break; }
      }
      if (!found && val) input.appendChild(new Option(val, val));
      input.value = val;
    } else {
      input.value = parts.join(', ');
    }
  }

  function mountStyleChips(container) {
    if (!container || container.getAttribute('data-mounted') === '1') return;
    container.setAttribute('data-mounted', '1');
    container.textContent = '';
    Object.keys(STYLE_GROUPS).forEach((key) => {
      const title = key === 'genres' ? 'Жанры' : key === 'moods' ? 'Настроение' : 'Голос';
      container.appendChild(Object.assign(document.createElement('p'), {
        className: 'text-[11px] font-semibold uppercase tracking-widest text-zinc-500 mt-3 mb-2',
        textContent: title,
      }));
      const row = document.createElement('div');
      row.className = 'flex flex-wrap gap-2';
      STYLE_GROUPS[key].forEach((label) => {
        const b = document.createElement('button');
        b.type = 'button';
        b.className = 'style-chip text-[11px] px-2.5 py-1.5 rounded-full border border-white/10 text-zinc-400';
        b.setAttribute('data-style', label);
        b.textContent = label;
        b.addEventListener('click', () => {
          b.classList.toggle('is-on');
          b.classList.toggle('border-emerald-500/50', b.classList.contains('is-on'));
          b.classList.toggle('text-emerald-400', b.classList.contains('is-on'));
          b.classList.toggle('bg-emerald-500/10', b.classList.contains('is-on'));
          syncStyleField(container.closest('form') || document);
        });
        row.appendChild(b);
      });
      container.appendChild(row);
    });
  }

  function init() {
    applyMiniAppLayout();
    bindNav();
    mountStyleChips(qs('#style-chips'));
    initFromQuery();
    let n = 0;
    const t = setInterval(() => {
      n += 1;
      const before = document.body.classList.contains('uvo-miniapp');
      applyMiniAppLayout();
      const after = document.body.classList.contains('uvo-miniapp');
      if (after && !before) {
        try {
          if (!new URL(location.href).searchParams.get('view')) {
            setView('hub', { scroll: false, smooth: false });
          }
        } catch (_) {
          setView('hub', { scroll: false, smooth: false });
        }
      }
      if (n >= 25) clearInterval(t);
    }, 200);
  }

  global.UVOApp = { setView, init, mountStyleChips, syncStyleField, STYLE_GROUPS, goLocal, openTarget };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})(window);
