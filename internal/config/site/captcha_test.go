package site

import "testing"

func TestInteractiveCaptchaOptions(t *testing.T) {
	t.Parallel()

	cfg := &Site{
		Login:    &Login{TextClickCaptcha: true},
		Register: &Register{SlideCaptcha: true},
	}

	tests := []struct {
		name  string
		cfg   *Site
		scene string
		want  CaptchaOptions
	}{
		{"login", cfg, "login", CaptchaOptions{Click: true}},
		{"register", cfg, "register", CaptchaOptions{Slide: true}},
		{"missing config", nil, "login", CaptchaOptions{}},
		{"missing sections", &Site{}, "register", CaptchaOptions{}},
		{"unknown scene", cfg, "unknown", CaptchaOptions{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.cfg.InteractiveCaptchaOptions(tt.scene); got != tt.want {
				t.Errorf("options = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCaptchaOptionsSelectType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options CaptchaOptions
		allowed map[string]bool
	}{
		{"disabled", CaptchaOptions{}, map[string]bool{"none": true}},
		{"click only", CaptchaOptions{Click: true}, map[string]bool{"click": true}},
		{"slide only", CaptchaOptions{Slide: true}, map[string]bool{captchaSlide: true}},
		{"both", CaptchaOptions{Click: true, Slide: true}, map[string]bool{"click": true, captchaSlide: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for range 100 {
				if got := tt.options.SelectType(); !tt.allowed[got] {
					t.Fatalf("selected disabled type %q", got)
				}
			}
		})
	}
}
