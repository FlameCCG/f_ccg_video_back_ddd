package media

import mediadomain "f_ccg_video_back_ddd/internal/domain/media"

// ArtifactPaths resolves storage representations without exposing their format to the domain.
type ArtifactPaths interface {
	ValidateSourceTargetIsolation(source string, targets []mediadomain.TranscodeTarget) error
	SourceTargetMatchesInput(source, relativePath string) bool
	ShouldDeleteOriginalSource(initial, current string) bool
}
