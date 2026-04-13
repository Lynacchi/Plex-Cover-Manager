package cover

import "testing"

func TestParseCoverFile(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		title  string
		year   string
		season bool
		number int
	}{
		{
			name:  "series main",
			path:  `C:\covers\The Show (2024).png`,
			title: "The Show",
			year:  "2024",
		},
		{
			name:   "season word",
			path:   `C:\covers\The Show (2024) - Season 01.jpg`,
			title:  "The Show",
			year:   "2024",
			season: true,
			number: 1,
		},
		{
			name:   "specials",
			path:   `C:\covers\The Show (2024) - Specials.webp`,
			title:  "The Show",
			year:   "2024",
			season: true,
			number: 0,
		},
		{
			name:   "s notation",
			path:   `C:\covers\The Show (2024) - S02.jpeg`,
			title:  "The Show",
			year:   "2024",
			season: true,
			number: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseCoverFile(tt.path)
			if err != nil {
				t.Fatalf("ParseCoverFile() error = %v", err)
			}
			if parsed.Title != tt.title || parsed.Year != tt.year || parsed.IsSeason != tt.season || parsed.SeasonNumber != tt.number {
				t.Fatalf("parsed = %#v", parsed)
			}
		})
	}
}

func TestParseCoverFileRejectsUnsupportedExtension(t *testing.T) {
	if _, err := ParseCoverFile(`C:\covers\Movie (2020).gif`); err == nil {
		t.Fatal("expected unsupported extension error")
	}
}
