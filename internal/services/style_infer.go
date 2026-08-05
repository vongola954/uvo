package services

import (
	"strings"
	"unicode/utf8"
)

// InferStyleFromText picks a production style from idea/lyrics.
// Used at create-time so the user does not pick genre chips manually.
func InferStyleFromText(prompt, lyrics string) string {
	text := strings.ToLower(strings.TrimSpace(prompt + "\n" + lyrics))
	if text == "" {
		return "Поп-музыка, мужской вокал"
	}
	if utf8.RuneCountInString(text) > 4000 {
		text = string([]rune(text)[:4000])
	}

	type cand struct {
		style string
		score int
	}
	cands := []cand{
		{"Хип-хоп и R&B, мужской вокал", scoreHits(text, "рэп", "реп", "hip-hop", "hip hop", "хип-хоп", "trap", "бит", "flow", "рифм", "куплеты рэп")},
		{"Поп-музыка, женский вокал", scoreHits(text, "поп", "припев", "девоч", "девушка", "любовь", "сердц", "роман")},
		{"Поп-музыка, мужской вокал", scoreHits(text, "парень", "мужск", "брат", "друг")},
		{"Рок и метал", scoreHits(text, "рок", "гитар", "металл", "metal", "крик", "драйв", "бунт")},
		{"Электронная музыка", scoreHits(text, "synth", "синтез", "неон", "клуб", "танц", "электрон", "techno", "house", "edm", "бит")},
		{"Шансон", scoreHits(text, "шансон", "тюрем", "блатн", "судьба", "дорога дальняя")},
		{"Джаз и блюз", scoreHits(text, "джаз", "блюз", "jazz", "blues", "саксофон", "свинг")},
		{"Кантри", scoreHits(text, "кантри", "country", "ранчо", "ковб", "гитара акуст")},
		{"Фолк", scoreHits(text, "фолк", "народн", "folk", "балалай", "хоровод")},
		{"Регги", scoreHits(text, "регги", "reggae", "ямайк", "раст")},
		{"Классика", scoreHits(text, "классик", "оркестр", "симфон", "опер")},
		{"Детские", scoreHits(text, "детск", "малыш", "сказк", "колыбель", "игруш")},
	}

	// Mood / voice overlays
	mood := ""
	switch {
	case scoreHits(text, "груст", "слёз", "тоск", "боль", "один", "проща") >= 2:
		mood = "Грустное"
	case scoreHits(text, "люб", "нежн", "поцел", "роман") >= 2:
		mood = "Романтическое"
	case scoreHits(text, "энерг", "огн", "громк", "танц", "драйв") >= 2:
		mood = "Энергичное"
	case scoreHits(text, "ноч", "лун", "неон", "тишин") >= 2:
		mood = "Тёмное"
	case scoreHits(text, "мечт", "небо", "звёзд", "звезд") >= 2:
		mood = "Мечтательное"
	case scoreHits(text, "чилл", "спокой", "мягк") >= 1:
		mood = "Чилл"
	}

	voice := ""
	switch {
	case scoreHits(text, "женск", "девушка", "она ", "девоч", "леди") >= 1 && scoreHits(text, "мужск", "парень", "он ") == 0:
		voice = "Женский вокал"
	case scoreHits(text, "дуэт", "мы с тобой", "двое") >= 1:
		voice = "Дуэт"
	case scoreHits(text, "детск", "ребён", "ребенок") >= 1:
		voice = "Детский вокал"
	case scoreHits(text, "мужск", "парень", "он ") >= 1:
		voice = "Мужской вокал"
	}

	best := cands[1] // default pop female-ish; overridden below
	best.score = -1
	for _, c := range cands {
		if c.score > best.score {
			best = c
		}
	}
	if best.score <= 0 {
		// Night/electronic vibe without strong genre hits
		if scoreHits(text, "ноч", "неон", "город", "дожд") >= 2 {
			best.style = "Электронная музыка, Чилл"
		} else if scoreHits(text, "люб", "сердц") >= 1 {
			best.style = "Поп-музыка, Романтическое"
		} else {
			best.style = "Поп-музыка"
		}
	}

	parts := []string{best.style}
	// Avoid duplicating voice/mood already in best.style
	low := strings.ToLower(best.style)
	if mood != "" && !strings.Contains(low, strings.ToLower(mood)) {
		parts = append(parts, mood)
	}
	if voice != "" && !strings.Contains(low, strings.ToLower(voice)) {
		parts = append(parts, voice)
	}
	out := strings.Join(parts, ", ")
	if utf8.RuneCountInString(out) > 200 {
		out = string([]rune(out)[:200])
	}
	return out
}

func scoreHits(text string, words ...string) int {
	n := 0
	for _, w := range words {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" {
			continue
		}
		if strings.Contains(text, w) {
			n++
		}
	}
	return n
}
