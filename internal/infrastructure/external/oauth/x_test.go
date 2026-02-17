package oauth

import (
	"encoding/json"
	"testing"
)

func TestNormalizeXTokenBody_HandlesBase64EncodedJSON(t *testing.T) {
	encoded := []byte("eyJ0b2tlbl90eXBlIjoiYmVhcmVyIiwiZXhwaXJlc19pbiI6NzIwMCwiYWNjZXNzX3Rva2VuIjoiY1U1eE1HUTVZa3RtY2xvek9GTklORlo2ZEdSWGVIWmZOekZMWkRaeVpGaG5jMVZCVHpGUVJHVmpkMjFNT2pFM056VTRNVEk0T0RFNU5EQTZNVG93T21GME9qRSIsInNjb3BlIjoidXNlcnMuZW1haWwgdXNlcnMucmVhZCJ9")

	normalized := normalizeXTokenBody(encoded)

	var token XAccessToken
	if err := json.Unmarshal(normalized, &token); err != nil {
		t.Fatalf("unmarshal normalized token failed: %v", err)
	}

	if token.AccessToken == "" {
		t.Fatalf("expected access token parsed from base64 body")
	}

	if token.Scope != "users.email users.read" {
		t.Fatalf("unexpected scope: %s", token.Scope)
	}
}

func TestNormalizeXTokenBody_LeavesJSONUntouched(t *testing.T) {
	body := []byte(`{"token_type":"bearer","expires_in":7200,"access_token":"token","scope":"users.read"}`)

	normalized := normalizeXTokenBody(body)

	if string(normalized) != string(body) {
		t.Fatalf("expected plain json body unchanged, got %s", string(normalized))
	}
}
