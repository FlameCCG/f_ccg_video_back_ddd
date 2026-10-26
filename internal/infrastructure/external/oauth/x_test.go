package oauth

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeXTokenBody_HandlesBase64EncodedJSON(t *testing.T) {
	t.Parallel()

	encoded := []byte("eyJ0b2tlbl90eXBlIjoiYmVhcmVyIiwiZXhwaXJlc19pbiI6NzIwMCwiYWNjZXNzX3Rva2VuIjoiY1U1eE1HUTVZa3RtY2xvek9GTklORlo2ZEdSWGVIWmZOekZMWkRaeVpGaG5jMVZCVHpGUVJHVmpkMjFNT2pFM056VTRNVEk0T0RFNU5EQTZNVG93T21GME9qRSIsInNjb3BlIjoidXNlcnMuZW1haWwgdXNlcnMucmVhZCJ9")

	normalized := normalizeXTokenBody(encoded)

	var token XAccessToken
	{
		err := json.Unmarshal(normalized, &token)
		require.NoError(t, err, "unmarshal normalized token failed: %v", err)
	}

	require.NotEmpty(t, token.AccessToken, "expected access token parsed from base64 body")

	require.Equal(t, "users.email users.read", token.Scope, "unexpected scope: %s", token.Scope)
}

func TestNormalizeXTokenBody_LeavesJSONUntouched(t *testing.T) {
	t.Parallel()

	body := []byte(`{"token_type":"bearer","expires_in":7200,"access_token":"token","scope":"users.read"}`)

	normalized := normalizeXTokenBody(body)

	require.Equal(t, string(normalized), string(body), "expected plain json body unchanged, got %s", string(normalized))
}
