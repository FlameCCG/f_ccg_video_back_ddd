package clienterror

import (
	"errors"
	"f_ccg_video_back_ddd/pkg/apperr"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	domainidentity "f_ccg_video_back_ddd/internal/domain/identity"

	video "f_ccg_video_back_ddd/internal/domain/video"
)

func TestMessageDoesNotExposeInfrastructureOrWrappedDetails(t *testing.T) {
	t.Parallel()

	for _, err := range []error{errors.New("SELECT password FROM users at 10.0.0.1"), fmt.Errorf("private-object-key: %w", video.ErrVideoNotFound)} {
		got := Message(err)
		require.NotContains(t, got, "password", "leaked details: %s", got)
		require.NotContains(t, got, "private-object", "leaked details: %s", got)
		require.NotContains(t, got, "10.0.", "leaked details: %s", got)
	}

	{
		got := Message(fmt.Errorf("database detail: %w", video.ErrVideoNotFound))
		require.Equal(t, got, video.ErrVideoNotFound.Error(), "lost business message: %s", got)
	}
}

func TestOuterSemanticsOverrideSafeBusinessDescriptors(t *testing.T) {
	t.Parallel()

	err := apperr.Wrap("lookup", domainidentity.ErrUserNotFound, errors.New("database unavailable"), apperr.Dependency)
	require.Equal(t, Message(err), domainidentity.ErrUserNotFound.Error(), "safe response message changed")

	description := Describe(err)
	require.Equal(t, apperr.Dependency, description.Kind, "allowlisted message suppressed dependency classification: %+v", description)

	{
		got := Describe(errors.New("private unknown failure"))
		require.Equal(t, apperr.Internal, got.Kind, "unknown failure=%+v", got)
		require.Equal(t, "internal.unknown", got.Code, "unknown failure=%+v", got)
	}
}

func TestSafeVisibilityCannotHideUnknownCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("database unavailable")
	for _, err := range []error{apperr.Wrap("lookup", video.ErrVideoNotFound, cause), fmt.Errorf("%w: %w", video.ErrVideoNotFound, cause)} {
		require.Equal(t, Message(err), video.ErrVideoNotFound.Error(), "response compatibility changed")

		{
			description := Describe(err)
			require.Equal(t, apperr.Internal, description.Kind, "system cause hidden: %+v", description)
		}
	}

	expected := apperr.Wrap("lookup", video.ErrVideoNotFound, fmt.Errorf("repository: %w", video.ErrVideoNotFound))
	{
		description := Describe(expected)
		require.Equal(t, apperr.NotFound, description.Kind, "expected missing resource promoted: %+v", description)
	}
}

func TestJoinedSystemCauseOutranksExpectedOrSecurityBranch(t *testing.T) {
	t.Parallel()

	for _, kind := range []apperr.Kind{apperr.Rejected, apperr.Security} {
		expected := apperr.Wrap("business", nil, errors.New("refusal"), kind)

		joined := errors.Join(expected, errors.New("system unavailable"))
		{
			description := Describe(joined)
			require.Equal(t, apperr.Internal, description.Kind, "joined system cause hidden by %s: %+v", kind, description)
		}
	}
}
