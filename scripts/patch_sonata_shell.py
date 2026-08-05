# -*- coding: utf-8 -*-
from pathlib import Path

path = Path(__file__).resolve().parents[1] / "internal/api/web/static/index.html"
html = path.read_text(encoding="utf-8")

css = """
    body.uvo-miniapp .uvo-marketing { display: none !important; }
    body.uvo-miniapp, body:not(.uvo-miniapp) { padding-bottom: 5.5rem; }
    .bottom-nav {
      position: fixed; left: 0; right: 0; bottom: 0; z-index: 60;
      display: grid; grid-template-columns: repeat(4, 1fr);
      gap: 2px; padding: 8px 10px calc(8px + env(safe-area-inset-bottom));
      background: rgba(5,5,7,0.92); border-top: 1px solid var(--border);
      backdrop-filter: blur(18px);
    }
    .bottom-nav a, .bottom-nav button {
      display: flex; flex-direction: column; align-items: center; justify-content: center;
      gap: 2px; padding: 8px 4px; border-radius: 14px; font-size: 10px; font-weight: 600;
      color: #71717a; background: transparent; border: 0; text-decoration: none;
    }
    .bottom-nav .uvo-nav-active { color: var(--accent); background: var(--accent-dim); }
    .bottom-nav .nav-ico { font-size: 16px; line-height: 1; }
    .hub-card { text-align: left; width: 100%; transition: border-color .2s, transform .2s; }
    .hub-card:hover { border-color: rgba(52,211,153,0.35); transform: translateY(-2px); }
    .style-chip.is-on { border-color: rgba(52,211,153,0.5); color: #34d399; background: rgba(52,211,153,0.1); }
    .dual-grid { display: grid; gap: 12px; }
    @media (min-width: 640px) { .dual-grid.has-alt { grid-template-columns: 1fr 1fr; } }
"""
if ".bottom-nav" not in html:
    html = html.replace("  </style>", css + "\n  </style>", 1)

hub = r"""
    <!-- App hub (Sonata-like menu, UVO design) -->
    <section data-uvo-view="hub" class="px-5 pt-20 pb-8 hidden">
      <div class="max-w-2xl mx-auto">
        <div class="flex items-end justify-between mb-5">
          <div>
            <p class="chip mb-3"><span class="pulse-dot"></span> 2 песни в подарок · от 99 ₽</p>
            <h1 class="text-3xl font-black tracking-tight">UVO<span style="color:var(--accent)">.</span></h1>
            <p class="text-sm text-zinc-500 mt-1">Создай песню · своим голосом · минус · портрет</p>
          </div>
          <span id="credits-chip-hub" class="text-xs text-zinc-500"></span>
        </div>
        <button type="button" data-uvo-open="create" class="btn-primary w-full py-4 rounded-2xl font-bold mb-4">Создать песню бесплатно</button>
        <div class="grid gap-2">
          <button type="button" data-uvo-open="create" class="glass hub-card p-4">
            <div class="text-sm font-semibold text-emerald-400 mb-1">Создать песню</div>
            <p class="text-xs text-zinc-500">Идея · свой текст · instrumental. AceMusic / Suno.</p>
          </button>
          <button type="button" data-uvo-open="lyrics" class="glass hub-card p-4">
            <div class="text-sm font-semibold text-emerald-400 mb-1">Создать стихи</div>
            <p class="text-xs text-zinc-500">Черновик текста по идее — бесплатно, затем в песню.</p>
          </button>
          <button type="button" data-uvo-open="create:voice" class="glass hub-card p-4">
            <div class="text-sm font-semibold text-emerald-400 mb-1">Песня твоим голосом</div>
            <p class="text-xs text-zinc-500">Клон голоса → генерация и каверы.</p>
          </button>
          <button type="button" data-uvo-open="create:cover" class="glass hub-card p-4">
            <div class="text-sm font-semibold text-emerald-400 mb-1">Кавер</div>
            <p class="text-xs text-zinc-500">Любой трек своим клонированным голосом (−2).</p>
          </button>
          <button type="button" data-uvo-open="tracks" class="glass hub-card p-4">
            <div class="text-sm font-semibold text-emerald-400 mb-1">Минусовка / портрет</div>
            <p class="text-xs text-zinc-500">Стемы, караоке и singing portrait из ваших треков.</p>
          </button>
          <button type="button" data-uvo-open="balance" class="glass hub-card p-4">
            <div class="text-sm font-semibold text-emerald-400 mb-1">Баланс и тарифы</div>
            <p class="text-xs text-zinc-500">Пакеты от 99 ₽ · цена за песню как у рынка.</p>
          </button>
        </div>
        <div class="mt-4 flex flex-wrap gap-2">
          <a href="#showcase" class="btn-ghost text-xs px-3 py-2 rounded-xl uvo-marketing">Витрина</a>
          <a href="/feed.html" class="btn-ghost text-xs px-3 py-2 rounded-xl">Лента</a>
          <a href="/playlists.html" class="btn-ghost text-xs px-3 py-2 rounded-xl">Плейлисты</a>
        </div>
      </div>
    </section>

    <section data-uvo-view="lyrics" class="px-5 pt-20 pb-8 hidden">
      <div class="max-w-2xl mx-auto glass p-6 sm:p-8">
        <h2 class="text-xl font-bold mb-1">Создать стихи</h2>
        <p class="text-sm text-zinc-500 mb-5">Опишите идею — получите черновик куплетов и припева. Текст можно править и сразу отправить в генерацию.</p>
        <label class="block text-[11px] font-semibold uppercase tracking-widest text-zinc-500 mb-2">Идея стихов</label>
        <textarea id="lyrics-idea" rows="4" placeholder="Поздравление маме с днём рождения, тепло, благодарность, лёгкий поп"></textarea>
        <label class="block text-[11px] font-semibold uppercase tracking-widest text-zinc-500 mb-2 mt-4">Стиль (опц.)</label>
        <input id="lyrics-style" type="text" placeholder="лирический поп, женский вокал">
        <button type="button" id="btn-write-lyrics" class="btn-primary w-full py-3.5 rounded-2xl font-bold mt-4">Написать стихи</button>
        <p id="lyrics-assist-status" class="text-xs text-zinc-500 mt-2"></p>
        <label class="block text-[11px] font-semibold uppercase tracking-widest text-zinc-500 mb-2 mt-5">Текст</label>
        <textarea id="lyrics-draft" rows="10" placeholder="Здесь появится текст…"></textarea>
        <button type="button" id="btn-lyrics-to-song" class="btn-ghost w-full py-3 rounded-2xl font-semibold mt-3">Использовать для песни →</button>
      </div>
    </section>

"""

if "App hub (Sonata-like menu" not in html:
    html = html.replace("    <!-- Studio glass panel -->", hub + "    <!-- Studio glass panel -->", 1)

replacements = [
    ('<section class="pt-28 pb-16 px-5">', '<section class="pt-28 pb-16 px-5 uvo-marketing">'),
    ('<section id="presets" class="px-5 pb-14">', '<section id="presets" class="uvo-marketing px-5 pb-14">'),
    ('<section id="showcase" class="px-5 pb-16">', '<section id="showcase" class="uvo-marketing px-5 pb-16">'),
    ('<section id="features" class="px-5 pb-20">', '<section id="features" class="uvo-marketing px-5 pb-20">'),
    ('<footer class="border-t', '<footer class="uvo-marketing border-t'),
    ('<section id="generate" class="px-5 pb-20">', '<section id="generate" data-uvo-view="create" class="px-5 pt-20 pb-8">'),
    ('<section id="pricing" class="px-5 pb-24">', '<section id="pricing" data-uvo-view="balance" class="px-5 pt-20 pb-8">'),
    (">Instrumental</button>", ">Инструментал</button>"),
    (">Идея</button>", ">Идея песни</button>"),
    ("GENERATE · −1", "Создать песню · 1 кредит"),
    ('function setGenMode(mode) {', "window.setGenMode = function setGenMode(mode) {"),
    (
        '<script src="/static/auth.js?v=2.8.6"></script>',
        '<script src="/static/auth.js?v=2.9.0"></script>\n  <script src="/static/app-shell.js?v=2.9.0"></script>',
    ),
]
for a, b in replacements:
    if a in html:
        html = html.replace(a, b, 1)

inject_form = """
          <div id="field-title" class="hidden">
            <label class="block text-[11px] font-semibold uppercase tracking-widest text-zinc-500 mb-2">Название</label>
            <input id="track-title-input" type="text" maxlength="80" placeholder="Название песни (опц.)">
          </div>
          <div id="idea-draft-wrap" class="flex items-center gap-3 text-xs text-zinc-400">
            <input type="checkbox" id="idea-draft-first" class="rounded border-white/20">
            <label for="idea-draft-first">Сначала черновик текста с правками</label>
          </div>
          <div id="style-chips-wrap" class="hidden">
            <p class="text-[11px] font-semibold uppercase tracking-widest text-zinc-500 mb-1">Стили</p>
            <div id="style-chips"></div>
            <input id="style-free" type="text" class="mt-3" placeholder="Свой стиль / теги (опц.)" maxlength="1000">
          </div>
"""
if "idea-draft-first" not in html:
    html = html.replace(
        '<input type="hidden" id="instrumental" value="0">',
        inject_form + '<input type="hidden" id="instrumental" value="0">',
        1,
    )

old_result = """        <div id="result" class="mt-6 hidden">
          <div class="player-card rounded-2xl p-5">
            <p id="track-title" class="font-semibold text-emerald-400 mb-3 text-sm"></p>
            <audio id="player" controls class="w-full"></audio>
            <div class="flex flex-wrap gap-2 mt-4">
              <a id="dl-link" href="#" class="btn-ghost text-xs px-4 py-2 rounded-xl">Скачать mp3</a>
              <a id="stems-link" href="/tracks.html" class="btn-ghost text-xs px-4 py-2 rounded-xl">Минусовка (−2)</a>
              <button type="button" id="btn-close-result" class="btn-ghost text-xs px-4 py-2 rounded-xl">Закрыть</button>
            </div>
          </div>
        </div>"""
new_result = """        <div id="result" class="mt-6 hidden">
          <p class="text-xs text-zinc-500 mb-2">Готово — сравните варианты и скачайте лучший</p>
          <div id="dual-grid" class="dual-grid">
            <div class="player-card rounded-2xl p-5">
              <p class="text-[10px] uppercase tracking-wider text-zinc-500 mb-1">Вариант 1</p>
              <p id="track-title" class="font-semibold text-emerald-400 mb-3 text-sm"></p>
              <audio id="player" controls class="w-full"></audio>
              <div class="flex flex-wrap gap-2 mt-4">
                <a id="dl-link" href="#" class="btn-ghost text-xs px-4 py-2 rounded-xl">Скачать</a>
                <a id="stems-link" href="/tracks.html" class="btn-ghost text-xs px-4 py-2 rounded-xl">Минус (−2)</a>
              </div>
            </div>
            <div id="alt-card" class="player-card rounded-2xl p-5 hidden">
              <p class="text-[10px] uppercase tracking-wider text-zinc-500 mb-1">Вариант 2</p>
              <p id="alt-track-title" class="font-semibold text-emerald-400 mb-3 text-sm">Вариант 2</p>
              <audio id="alt-player" controls class="w-full"></audio>
              <div class="flex flex-wrap gap-2 mt-4">
                <a id="alt-dl-link" href="#" class="btn-ghost text-xs px-4 py-2 rounded-xl">Скачать</a>
              </div>
            </div>
          </div>
          <button type="button" id="btn-close-result" class="btn-ghost text-xs px-4 py-2 rounded-xl mt-3">Закрыть</button>
        </div>"""
if "dual-grid" not in html and old_result in html:
    html = html.replace(old_result, new_result, 1)

bottom = """
    <nav class="bottom-nav" aria-label="Основная навигация">
      <button type="button" data-uvo-nav="lyrics"><span class="nav-ico">✎</span>Стихи</button>
      <button type="button" data-uvo-nav="create"><span class="nav-ico">♪</span>Создать</button>
      <a href="/tracks.html" data-uvo-nav="tracks"><span class="nav-ico">☰</span>Треки</a>
      <button type="button" data-uvo-nav="balance"><span class="nav-ico">◇</span>Баланс</button>
    </nav>
"""
if "bottom-nav" not in html:
    html = html.replace(
        '  </div>\n\n  <script src="/static/auth.js',
        bottom + '  </div>\n\n  <script src="/static/auth.js',
        1,
    )

# mode UI block
needle = """      if (genMode === 'lyrics') {
        if (hint) hint.textContent = 'Вставьте свои стихи — стиль и голос опционально. −1 кредит.';
        if (lyricsLab) lyricsLab.textContent = 'Стихи (обязательно)';
        if (promptLab) promptLab.textContent = 'Промпт / настроение (опц.)';
        if (fieldLyrics) fieldLyrics.classList.remove('hidden');
      } else if (genMode === 'instrumental') {
        if (hint) hint.textContent = 'Только минус без вокала. Нужен промпт или стиль. −1 кредит.';
        if (lyricsLab) lyricsLab.textContent = 'Стихи';
        if (promptLab) promptLab.textContent = 'Промпт / настроение';
        if (fieldLyrics) fieldLyrics.classList.add('hidden');
      } else {
        if (hint) hint.textContent = 'Опишите настроение и звук — Suno напишет трек. −1 кредит ≈ 1 песня.';
        if (lyricsLab) lyricsLab.textContent = 'Стихи (опц.)';
        if (promptLab) promptLab.textContent = 'Промпт';
        if (fieldLyrics) fieldLyrics.classList.remove('hidden');
      }"""
repl = """      const chips = document.getElementById('style-chips-wrap');
      const titleField = document.getElementById('field-title');
      const draftWrap = document.getElementById('idea-draft-wrap');
      if (genMode === 'lyrics') {
        if (hint) hint.textContent = 'Вставьте свои стихи — теги стиля ниже. −1 кредит.';
        if (lyricsLab) lyricsLab.textContent = 'Текст (обязательно)';
        if (promptLab) promptLab.textContent = 'Описание / настроение (опц.)';
        if (fieldLyrics) fieldLyrics.classList.remove('hidden');
        if (chips) chips.classList.remove('hidden');
        if (titleField) titleField.classList.remove('hidden');
        if (draftWrap) draftWrap.classList.add('hidden');
        if (window.UVOApp) UVOApp.mountStyleChips(document.getElementById('style-chips'));
      } else if (genMode === 'instrumental') {
        if (hint) hint.textContent = 'Инструментал без вокала — описание или стили. −1 кредит.';
        if (lyricsLab) lyricsLab.textContent = 'Стихи';
        if (promptLab) promptLab.textContent = 'Описание';
        if (fieldLyrics) fieldLyrics.classList.add('hidden');
        if (chips) chips.classList.remove('hidden');
        if (titleField) titleField.classList.remove('hidden');
        if (draftWrap) draftWrap.classList.add('hidden');
        if (window.UVOApp) UVOApp.mountStyleChips(document.getElementById('style-chips'));
      } else {
        if (hint) hint.textContent = 'Опишите идею своими словами — нейросеть сочинит песню. −1 кредит.';
        if (lyricsLab) lyricsLab.textContent = 'Стихи (опц.)';
        if (promptLab) promptLab.textContent = 'Идея песни';
        if (fieldLyrics) fieldLyrics.classList.remove('hidden');
        if (chips) chips.classList.add('hidden');
        if (titleField) titleField.classList.add('hidden');
        if (draftWrap) draftWrap.classList.remove('hidden');
      }"""
if needle in html:
    html = html.replace(needle, repl, 1)
else:
    print("WARN: mode block not found")

extra_js = r"""
    // Lyrics studio + dual result + hub credits (Sonata-like flows)
    document.getElementById('btn-write-lyrics')?.addEventListener('click', async () => {
      const idea = document.getElementById('lyrics-idea')?.value || '';
      const style = document.getElementById('lyrics-style')?.value || '';
      const status = document.getElementById('lyrics-assist-status');
      if (!idea.trim()) { alert('Опишите идею стихов'); return; }
      const btn = document.getElementById('btn-write-lyrics');
      btn.disabled = true;
      if (status) status.textContent = 'Пишу черновик…';
      try {
        await UVO.ensureDevToken();
        const res = await UVO.api('/api/lyrics/assist', {
          method: 'POST',
          body: JSON.stringify({ idea, style }),
        });
        const d = await res.json().catch(() => ({}));
        if (!res.ok) throw new Error((d.error && d.error.message) || 'Нужен OPENAI_API_KEY');
        document.getElementById('lyrics-draft').value = d.lyrics || '';
        if (status) status.textContent = 'Готово — отредактируйте и отправьте в песню';
      } catch (e) {
        if (status) status.textContent = e.message || 'Ошибка';
        alert(e.message || 'Ошибка');
      } finally {
        btn.disabled = false;
      }
    });
    document.getElementById('btn-lyrics-to-song')?.addEventListener('click', () => {
      const text = document.getElementById('lyrics-draft')?.value || '';
      const idea = document.getElementById('lyrics-idea')?.value || '';
      const style = document.getElementById('lyrics-style')?.value || '';
      if (!text.trim()) { alert('Сначала получите или вставьте текст'); return; }
      if (window.UVOApp) UVOApp.setView('create', { mode: 'lyrics' });
      else location.hash = '#generate';
      setGenMode('lyrics');
      const ly = document.getElementById('lyrics');
      const pr = document.getElementById('prompt');
      const st = document.getElementById('style');
      if (ly) ly.value = text;
      if (pr && idea) pr.value = idea;
      if (st && style) {
        let found = false;
        for (const o of st.options) if (o.value === style) found = true;
        if (!found) st.appendChild(UVO.el('option', { value: style, text: style }));
        st.value = style;
      }
    });

    const _showResult = typeof showResult === 'function' ? null : null;
"""

# Patch showResult function body markers
old_show = """      function showResult(playUrl, title, duration, downloadUrl, trackId, altPlayUrl) {
        document.getElementById('player').src = playUrl;
        let t = (title || 'Трек') + ' · ' + (duration || '?') + 's';
        if (altPlayUrl) t += ' · есть v2';
        document.getElementById('track-title').textContent = t;
        const dl = document.getElementById('dl-link');
        const dlUrl = downloadUrl || ('/tracks/' + trackId + '/download');
        dl.href = dlUrl;
        dl.onclick = async (e) => {
          e.preventDefault();
          try {
            await UVO.downloadFile(dlUrl, 'uvo-track-' + (trackId || 'x') + '.mp3');
          } catch (err) {
            alert(err.message || 'Не удалось скачать');
          }
        };
        const stems = document.getElementById('stems-link');
        if (stems && trackId) stems.href = '/karaoke.html?id=' + trackId;
        let alt = document.getElementById('alt-link');
        if (altPlayUrl) {
          if (!alt) {
            alt = document.createElement('a');
            alt.id = 'alt-link';
            alt.className = 'btn-ghost text-xs px-4 py-2 rounded-xl';
            alt.textContent = 'Вариант 2';
            document.getElementById('dl-link')?.parentElement?.appendChild(alt);
          }
          alt.href = altPlayUrl;
          alt.classList.remove('hidden');
          alt.onclick = (e) => { e.preventDefault(); document.getElementById('player').src = altPlayUrl; };
        } else if (alt) {
          alt.classList.add('hidden');
        }
        result.classList.remove('hidden');
      }"""

new_show = """      function showResult(playUrl, title, duration, downloadUrl, trackId, altPlayUrl, altTrackId) {
        document.getElementById('player').src = playUrl;
        const t = (title || 'Трек') + ' · ' + (duration || '?') + 's';
        document.getElementById('track-title').textContent = t;
        const dl = document.getElementById('dl-link');
        const dlUrl = downloadUrl || (trackId ? ('/tracks/' + trackId + '/download') : playUrl);
        dl.href = dlUrl;
        dl.onclick = async (e) => {
          e.preventDefault();
          try { await UVO.downloadFile(dlUrl, 'uvo-track-' + (trackId || 'x') + '.mp3'); }
          catch (err) { alert(err.message || 'Не удалось скачать'); }
        };
        const stems = document.getElementById('stems-link');
        if (stems && trackId) stems.href = '/karaoke.html?id=' + trackId;
        const grid = document.getElementById('dual-grid');
        const altCard = document.getElementById('alt-card');
        const altPlayer = document.getElementById('alt-player');
        const altDl = document.getElementById('alt-dl-link');
        if (altPlayUrl && altCard && altPlayer) {
          altCard.classList.remove('hidden');
          if (grid) grid.classList.add('has-alt');
          altPlayer.src = altPlayUrl;
          document.getElementById('alt-track-title').textContent = (title || 'Трек') + ' · v2';
          const altUrl = altTrackId ? ('/tracks/' + altTrackId + '/download') : altPlayUrl;
          if (altDl) {
            altDl.href = altUrl;
            altDl.onclick = async (e) => {
              e.preventDefault();
              try { await UVO.downloadFile(altUrl, 'uvo-track-' + (altTrackId || 'alt') + '.mp3'); }
              catch (err) { alert(err.message || 'Не удалось скачать'); }
            };
          }
        } else if (altCard) {
          altCard.classList.add('hidden');
          if (grid) grid.classList.remove('has-alt');
        }
        result.classList.remove('hidden');
        if (window.UVOApp) UVOApp.setView('create', { scroll: false });
      }"""

if old_show in html:
    html = html.replace(old_show, new_show, 1)
else:
    print("WARN: showResult not found exactly")

# Update job showResult call to pass alt track id
html = html.replace(
    """              showResult(
                job.play_url || job.PlayURL,
                job.title || job.Title,
                job.duration || job.Duration,
                job.download_url || job.DownloadURL,
                job.track_id || job.TrackID,
                job.alt_play_url || job.AltPlayURL
              );""",
    """              showResult(
                job.play_url || job.PlayURL,
                job.title || job.Title,
                job.duration || job.Duration,
                job.download_url || job.DownloadURL,
                job.track_id || job.TrackID,
                job.alt_play_url || job.AltPlayURL,
                job.alt_track_id || job.AltTrackID
              );""",
    1,
)

# idea-draft-first intercept before generate
draft_hook = """
      if (document.getElementById('idea-draft-first')?.checked && (document.getElementById('gen-mode')?.value || genMode) === 'idea') {
        const idea = document.getElementById('prompt')?.value || '';
        if (!idea.trim()) { alert('Опишите идею'); btn.disabled = false; return; }
        try {
          await UVO.ensureDevToken();
          const res = await UVO.api('/api/lyrics/assist', { method: 'POST', body: JSON.stringify({ idea, style: document.getElementById('style')?.value || '' }) });
          const d = await res.json().catch(() => ({}));
          if (!res.ok) throw new Error((d.error && d.error.message) || 'Нужен OPENAI_API_KEY для черновика');
          document.getElementById('lyrics').value = d.lyrics || '';
          setGenMode('lyrics');
          document.getElementById('idea-draft-first').checked = false;
          document.getElementById('status-text').textContent = 'Черновик готов — проверьте стихи и нажмите «Создать» ещё раз';
          stopProgress(false);
          btn.disabled = false;
          return;
        } catch (e) {
          stopProgress(false);
          document.getElementById('status-text').textContent = 'Ошибка: ' + e.message;
          btn.disabled = false;
          return;
        }
      }
"""
anchor = "      const mode = document.getElementById('gen-mode')?.value || genMode || 'idea';"
if "idea-draft-first')?.checked" not in html and anchor in html:
    html = html.replace(anchor, draft_hook + "\n" + anchor, 1)

# sync credits chip to hub
html = html.replace(
    "if (el && cr.ok) el.textContent = 'кредиты: ' + d.balance;",
    "if (el && cr.ok) el.textContent = 'кредиты: ' + d.balance;\n"
    "          const hub = document.getElementById('credits-chip-hub');\n"
    "          if (hub && cr.ok) hub.textContent = 'кредиты: ' + d.balance;",
)

# append lyrics handlers before closing script if not present
if "btn-write-lyrics" not in html.split("btn-write-lyrics")[0] or html.count("btn-write-lyrics") < 2:
    # insert before last </script> in file that follows auth - find "loadPresets();"
    if "btn-write-lyrics')?.addEventListener" not in html:
        html = html.replace(
            "    loadPresets();",
            """    document.getElementById('btn-write-lyrics')?.addEventListener('click', async () => {
      const idea = document.getElementById('lyrics-idea')?.value || '';
      const style = document.getElementById('lyrics-style')?.value || '';
      const status = document.getElementById('lyrics-assist-status');
      if (!idea.trim()) { alert('Опишите идею стихов'); return; }
      const btn = document.getElementById('btn-write-lyrics');
      btn.disabled = true;
      if (status) status.textContent = 'Пишу черновик…';
      try {
        await UVO.ensureDevToken();
        const res = await UVO.api('/api/lyrics/assist', { method: 'POST', body: JSON.stringify({ idea, style }) });
        const d = await res.json().catch(() => ({}));
        if (!res.ok) throw new Error((d.error && d.error.message) || 'Нужен OPENAI_API_KEY');
        document.getElementById('lyrics-draft').value = d.lyrics || '';
        if (status) status.textContent = 'Готово — отредактируйте и отправьте в песню';
      } catch (e) {
        if (status) status.textContent = e.message || 'Ошибка';
        alert(e.message || 'Ошибка');
      } finally { btn.disabled = false; }
    });
    document.getElementById('btn-lyrics-to-song')?.addEventListener('click', () => {
      const text = document.getElementById('lyrics-draft')?.value || '';
      const idea = document.getElementById('lyrics-idea')?.value || '';
      const style = document.getElementById('lyrics-style')?.value || '';
      if (!text.trim()) { alert('Сначала получите или вставьте текст'); return; }
      if (window.UVOApp) UVOApp.setView('create', { mode: 'lyrics' });
      setGenMode('lyrics');
      const ly = document.getElementById('lyrics');
      const pr = document.getElementById('prompt');
      const st = document.getElementById('style');
      if (ly) ly.value = text;
      if (pr && idea) pr.value = idea;
      if (st && style) {
        let found = false;
        for (const o of st.options) if (o.value === style) found = true;
        if (!found) st.appendChild(UVO.el('option', { value: style, text: style }));
        st.value = style;
      }
    });
    loadPresets();""",
            1,
        )

path.write_text(html, encoding="utf-8")
print("OK", path, "bytes", path.stat().st_size)
