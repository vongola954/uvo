package services

// ViralPreset is a one-click prompt+style pack (Soyuz-style growth UX).
type ViralPreset struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Emoji        string `json:"emoji,omitempty"`
	Category     string `json:"category"`
	Prompt       string `json:"prompt"`
	Style        string `json:"style"`
	LyricsHint   string `json:"lyrics_hint,omitempty"`
	Instrumental bool   `json:"instrumental,omitempty"`
}

// ViralPresets — готовые связки для лендинга и студии.
var ViralPresets = []ViralPreset{
	{ID: "night-drive", Title: "Ночной драйв", Emoji: "🌃", Category: "synth", Prompt: "ночной synth-pop, женский вокал, неоновый город после дождя, mid-tempo groove", Style: "Synthwave"},
	{ID: "gym-fire", Title: "Зал / motivation", Emoji: "🔥", Category: "energy", Prompt: "энергичный trap-phonk для тренировки, мощный drop, мужской вокал, агрессивный бит", Style: "Hip-Hop"},
	{ID: "sad-piano", Title: "Грустное пианино", Emoji: "🎹", Category: "ballad", Prompt: "медленная фортепианная баллада о расставании, тёплый мужской вокал, минимум инструментов", Style: "Ballad"},
	{ID: "tiktok-hook", Title: "TikTok хук", Emoji: "📱", Category: "viral", Prompt: "короткий цепляющий поп-хук на русском, танцевальный бит, яркий женский вокал, виральный припев", Style: "Pop"},
	{ID: "lofi-study", Title: "Lo-fi учёба", Emoji: "📚", Category: "chill", Prompt: "lo-fi hip-hop для учёбы, тёплый vinyl crackle, мягкий бит, без вокала", Style: "Lo-Fi", Instrumental: true},
	{ID: "wedding-vow", Title: "Свадебная", Emoji: "💍", Category: "occasion", Prompt: "нежная свадебная песня на русском, акустическая гитара, тёплый дуэт, счастливый финал", Style: "Acoustic"},
	{ID: "bday-fun", Title: "День рождения", Emoji: "🎂", Category: "occasion", Prompt: "весёлая поздравительная песня с днём рождения, поп, лёгкий юмор, припев который хочется петь", Style: "Pop"},
	{ID: "podcast-jingle", Title: "Джингл подкаста", Emoji: "🎙️", Category: "brand", Prompt: "короткий яркий джингл для подкаста, 15–30 сек ощущение, электронный поп, без длинных куплетов", Style: "Electronic", Instrumental: true},
	{ID: "game-boss", Title: "Босс игры", Emoji: "🎮", Category: "game", Prompt: "эпичный саундтрек битвы с боссом, оркестр + электроника, нарастающее напряжение", Style: "Cinematic", Instrumental: true},
	{ID: "reels-ugc", Title: "Reels UGC", Emoji: "✨", Category: "viral", Prompt: "лёгкий indie-pop для Reels, позитив, акустика и хлопки, женский вокал, хук в первые 3 секунды", Style: "Indie"},
	{ID: "rnb-night", Title: "R&B ночь", Emoji: "🌙", Category: "rnb", Prompt: "sensual R&B, мягкий мужской вокал, 808, городские огни, mid-tempo", Style: "R&B"},
	{ID: "rock-anthem", Title: "Рок-гимн", Emoji: "🎸", Category: "rock", Prompt: "драйвовый русский рок-гимн, электрогитары, мощный припев, стадионное чувство", Style: "Rock"},
	{ID: "deep-house", Title: "Deep house", Emoji: "🪩", Category: "dance", Prompt: "deep house для вечера, тёплый бас, женский вокал-хук, клубный но не агрессивный", Style: "House"},
	{ID: "kids-lullaby", Title: "Колыбельная", Emoji: "🧸", Category: "kids", Prompt: "нежная колыбельная, мягкий женский вокал, музыкальная шкатулка и тёплое пианино", Style: "Ambient"},
	{ID: "horror-trailer", Title: "Horror trailer", Emoji: "👻", Category: "cinematic", Prompt: "саундтрек для хоррор-трейлера, диссонанс, низкие удары, нарастающий страх", Style: "Cinematic", Instrumental: true},
	{ID: "rap-story", Title: "Мелодичный рэп", Emoji: "🎤", Category: "rap", Prompt: "мелодичный русский рэп о пути наверх, хуки, атмосферные пэды, уверенный флоу", Style: "Hip-Hop"},
	{ID: "jazz-cafe", Title: "Джаз-кафе", Emoji: "🎷", Category: "jazz", Prompt: "уютный jazz lounge, саксофон, контрабас, мягкий женский вокал, вечер в кафе", Style: "Jazz"},
	{ID: "metal-drop", Title: "Metal drop", Emoji: "⚡", Category: "metal", Prompt: "современный metalcore, тяжёлые риффы, чистый и скрим вокал, эпичный breakdown", Style: "Metal"},
	{ID: "folk-road", Title: "Дорога / folk", Emoji: "🌾", Category: "folk", Prompt: "русская folk-indie баллада о дороге домой, акустика, гармонь-намёк, тёплый вокал", Style: "Folk"},
	{ID: "edm-festival", Title: "EDM фест", Emoji: "🎧", Category: "dance", Prompt: "большой фестивальный EDM drop, euphoric lead, женский вокал в пре-дропе", Style: "EDM"},
	{ID: "asmr-ambient", Title: "Ambient сон", Emoji: "🌊", Category: "chill", Prompt: "ambient soundscape для сна, океан и мягкие пэды, без ритма и вокала", Style: "Ambient", Instrumental: true},
	{ID: "brand-jingle", Title: "Бренд-джингл", Emoji: "📣", Category: "brand", Prompt: "запоминающийся рекламный джингл на русском, дружелюбный поп, короткий яркий слоган в припеве", Style: "Pop"},
	{ID: "kpop-shine", Title: "K-pop shine", Emoji: "💫", Category: "pop", Prompt: "яркий k-pop dance track, мощный хук, смешанный вокал, глянцевая продакшн", Style: "K-Pop"},
	{ID: "blues-bar", Title: "Блюз-бар", Emoji: "🥃", Category: "blues", Prompt: "поздний electric blues, хриплый мужской вокал, гитарные соло, дымный бар", Style: "Blues"},
}

func PresetByID(id string) (ViralPreset, bool) {
	for _, p := range ViralPresets {
		if p.ID == id {
			return p, true
		}
	}
	return ViralPreset{}, false
}
