package rbac

import "testing"

func TestParseMenuSuggestion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{
			name: "valid json in code fence",
			text: "```json\n" +
				`{"titleEn":"Users","titleJa":"ユーザー","icon":"<svg viewBox=\"0 0 24 24\"></svg>",` +
				`"path":"users","name":"Users","component":"views/users/index","keepAlive":true,"sortOrder":2}` +
				"\n```",
		},
		{
			name: "reject event handler in svg",
			text: `{"titleEn":"Users","titleJa":"ユーザー","icon":"<svg onload=\"alert(1)\"></svg>",` +
				`"path":"users","name":"Users","component":"views/users/index","keepAlive":false,"sortOrder":1}`,
			wantErr: true,
		},
		{
			name:    "reject incomplete response",
			text:    `{"titleEn":"Users","sortOrder":1}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			suggestion, err := parseMenuSuggestion(tt.text)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parseMenuSuggestion() error = nil, want error")
				}

				return
			}

			if err != nil {
				t.Fatalf("parseMenuSuggestion() error = %v", err)
			}

			if suggestion.SortOrder != 2 || suggestion.Name != "Users" {
				t.Fatalf("unexpected suggestion: %#v", suggestion)
			}
		})
	}
}
