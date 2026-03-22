package mediaurl

import "testing"

func TestPlayableContentTypeURLPreservesStorageContract(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input, want string
	}{
		{"empty", "  ", ""},
		{"local path", " uploads/video.mp4 ", "uploads/video.mp4"},
		{"mp4", "https://media.example/v.mp4", "https://media.example/v.mp4?response-content-type=video%2Fmp4"},
		{"query", "https://media.example/v.MP4?token=abc%2F123", "https://media.example/v.MP4?token=abc%2F123&response-content-type=video%2Fmp4"},
		{"hls", "https://media.example/index.m3u8", "https://media.example/index.m3u8?response-content-type=application%2Fx-mpegURL"},
		{"segment", "https://media.example/p.ts", "https://media.example/p.ts?response-content-type=video%2Fmp2t"},
		{"already set", "https://media.example/v.mp4?response-content-type=custom", "https://media.example/v.mp4?response-content-type=custom"},
		{"escape path", "https://media.example/a b.mp4", "https://media.example/a%20b.mp4?response-content-type=video%2Fmp4"},
		{"other format", "https://media.example/a b.mpd", "https://media.example/a%20b.mpd"},
		{"dot in directory", "https://media.example/a.mp4/file", "https://media.example/a.mp4/file"},
		{"malformed", "https://media.example/%zz.mpd", "https://media.example/%zz.mpd"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			formatter := Formatter{}
			if got := formatter.PlayableContentTypeURL(tc.input); got != tc.want {
				t.Fatalf("URL = %q, want %q", got, tc.want)
			}
			if got := formatter.PlayableContentTypeURL(tc.want); got != tc.want {
				t.Fatalf("formatting twice changed URL: %q", got)
			}
		})
	}
}
