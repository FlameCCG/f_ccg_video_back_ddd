package video_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infra "f_ccg_video_back_ddd/internal/infrastructure/external/video"

	"go.uber.org/zap"
)

func TestTemplateDASHReportsActualTotalOfInitializationAndSegments(t *testing.T) {
	t.Parallel()

	sizes := map[string]int{"/init-0.mp4": 10, "/seg-0-00001.m4s": 21, "/seg-0-00002.m4s": 22, "/seg-0-00003.m4s": 23}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest.mpd" {
			_, _ = io.WriteString(w, `<MPD><Period><AdaptationSet contentType="video"><SegmentTemplate initialization="init-$RepresentationID$.mp4" media="seg-$RepresentationID$-$Number%05d$.m4s" startNumber="1"><SegmentTimeline><S d="2048" r="2"/></SegmentTimeline></SegmentTemplate><Representation id="0" bandwidth="2000000" height="720" width="1280"/></AdaptationSet></Period></MPD>`)
			return
		}

		size, ok := sizes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}

		assert.Equal(t, http.MethodHead, r.Method, "size probe used %s", r.Method)

		w.Header().Set("Content-Length", strconv.Itoa(size))
	}))
	defer server.Close()

	variants, err := infra.NewHTTPDASHVariantResolver(zap.NewNop()).Variants(t.Context(), 1, server.URL+"/manifest.mpd")
	require.NoError(t, err)

	require.Len(t, variants, 1, "variants=%+v", variants)
	require.EqualValues(t, 76, variants[0].FileSize, "variants=%+v", variants)
	require.Equal(t, "init-0.mp4", variants[0].FileName, "variants=%+v", variants)
}
