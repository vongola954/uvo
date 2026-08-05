/** UVO app shell — Sonata-like IA (bottom nav + views), UVO visual language */
(function (global) {
  const VIEWS = ['hub', 'lyrics', 'create', 'balance'];

  function qs(sel, root) {
    return (root || document).querySelector(sel);
  }

  function qsa(sel, root) {
    return Array.from((root || document).querySelectorAll(sel));
  }

  function setView(name, opts) {
    const view = VIEWS.includes(name) ? name : 'hub';
    qsa('[data-uvo-view]').forEach((el) => {
      el.classList.toggle('hidden', el.getAttribute('data-uvo-view') !== view);
    });
    qsa('[data-uvo-nav]').forEach((el) => {
      const on = el.getAttribute('data-uvo-nav') === view;
      el.classList.toggle('uvo-nav-active', on);
      el.setAttribute('aria-current', on ? 'page' : 'false');
    });
    document.body.setAttribute('data-uvo-view', view);
    if (opts && opts.scroll !== false) {
      window.scrollTo({ top: 0, behavior: opts.smooth === false ? 'auto' : 'smooth' });
    }
    try {
      const u = new URL(location.href);
      if (view === 'hub') u.searchParams.delete('view');
      else u.searchParams.set('view', view);
      history.replaceState({}, '', u.pathname + u.search + u.hash);
    } catch (_) {}
    if (view === 'create' && opts && opts.tab) {
      const tab = document.querySelector('.studio-tab[data-tab="' + opts.tab + '"]');
      if (tab) tab.click();
    }
    if (view === 'create' && opts && opts.mode && typeof global.setGenMode === 'function') {
      global.setGenMode(opts.mode);
    }
  }

  function bindNav() {
    qsa('[data-uvo-nav]').forEach((el) => {
      el.addEventListener('click', (e) => {
        const view = el.getAttribute('data-uvo-nav');
        if (!view) return;
        if (view === 'tracks') {
          // real page
          return;
        }
        e.preventDefault();
        setView(view);
      });
    });
    qsa('[data-uvo-open]').forEach((el) => {
      el.addEventListener('click', (e) => {
        e.preventDefault();
        const raw = el.getAttribute('data-uvo-open') || 'hub';
        const [view, extra] = raw.split(':');
        if (view === 'tracks') {
          location.href = '/tracks.html';
          return;
        }
        if (view === 'media') {
          const tab = extra || '';
          location.href = '/media.html' + (tab ? ('?tab=' + encodeURIComponent(tab)) : '');
          return;
        }
        if (view === 'distribution') {
          location.href = '/distribution.html';
          return;
        }
        if (view === 'playlists') {
          location.href = '/playlists.html';
          return;
        }
        if (view === 'feed') {
          location.href = '/feed.html';
          return;
        }
        const opts = {};
        if (extra === 'voice' || extra === 'cover' || extra === 'gen') opts.tab = extra === 'gen' ? 'gen' : extra;
        if (extra === 'idea' || extra === 'lyrics' || extra === 'instrumental') opts.mode = extra;
        setView(view, opts);
      });
    });
  }

  function applyMiniAppLayout() {
    const inMax = !!(global.UVO && global.UVO.inMaxWebApp && global.UVO.inMaxWebApp());
    document.body.classList.toggle('uvo-miniapp', inMax || location.search.indexOf('app=1') >= 0);
  }

  function initFromQuery() {
    try {
      const u = new URL(location.href);
      const v = u.searchParams.get('view');
      if (v && VIEWS.includes(v)) setView(v, { scroll: false, smooth: false });
      else if (document.body.classList.contains('uvo-miniapp')) setView('hub', { scroll: false, smooth: false });
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
      for (const o of input.options) {
        if (o.value === val) { found = true; break; }
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
  }

  global.UVOApp = { setView, init, mountStyleChips, syncStyleField, STYLE_GROUPS };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})(window);
