package services

// Predefined styles for generation
var StyleLibrary = []string{
	"Pop",
	"Rock",
	"Electronic",
	"Jazz",
	"Classical",
	"Rap",
	"Hip-Hop",
	"Folk",
	"Metal",
	"R&B",
	"Indie",
	"Ambient",
	"Dance",
	"Blues",
	"Country",
	"Reggae",
	"Punk",
	"Soul",
	"Funk",
	"Techno",
}

func IsValidStyle(s string) bool {
	for _, style := range StyleLibrary {
		if style == s {
			return true
		}
	}
	return false
}
