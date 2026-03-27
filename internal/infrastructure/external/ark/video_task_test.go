package ark

import (
	"encoding/json"
	"testing"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"
)

func TestVideoTaskRequestSerializesImageRoles(t *testing.T) {
	t.Parallel()
	for _, role := range []string{aiapp.VideoImageFirstFrame, aiapp.VideoImageReference} {
		t.Run(role, func(t *testing.T) {
			t.Parallel()
			request := buildVideoTaskRequest(aiapp.VideoInvocation{
				Model: "video-model", Prompt: "animate",
				ImageRefs: []aiapp.VideoImageRef{{URL: "https://example.com/image.png", Role: role}},
			})
			raw, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			var body struct {
				Content []struct {
					Type, Role string
					ImageURL   struct{ URL string } `json:"image_url"`
				}
			}
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Content) != 2 || body.Content[1].Role != role || body.Content[1].ImageURL.URL != "https://example.com/image.png" {
				t.Fatalf("request lost image role or URL: %s", raw)
			}
		})
	}
}
