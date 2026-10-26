package clienterror

import "f_ccg_video_back_ddd/pkg/apperr"

// causalDescriptor prevents a safe outer message from hiding an unknown system cause.
// Expected causes retain their descriptor; explicit outer semantics are handled by Describe first.
func causalDescriptor(err error) (Descriptor, bool) {
	if err == nil {
		return Descriptor{}, false
	}
	//nolint:errorlint // This exact node distinguishes the business and cause branches before recursion.
	if wrapper, ok := err.(*apperr.Error); ok {
		if wrapper.Business != nil && wrapper.Cause != nil {
			cause := Describe(wrapper.Cause)
			if outer, known := lookup(wrapper.Business); causalDescriptorCondition(cause, outer, known) {
				return outer, true
			}

			return cause, true
		}

		return causalDescriptor(wrapper.Cause)
	}

	if many, ok := err.(interface{ Unwrap() []error }); ok {
		var best Descriptor

		found := false

		for _, child := range many.Unwrap() {
			if child == nil {
				continue
			}

			description := Describe(child)
			if !found || classificationRank(description.Kind) > classificationRank(best.Kind) {
				best = description
				found = true
			}
		}

		return best, found
	}

	if one, ok := err.(interface{ Unwrap() error }); ok {
		return causalDescriptor(one.Unwrap())
	}

	return Descriptor{}, false
}

func classificationRank(kind apperr.Kind) int {
	switch kind {
	case apperr.Internal, apperr.Dependency:
		return 3
	case apperr.Security, apperr.Degraded:
		return 2
	case apperr.Validation, apperr.Rejected, apperr.NotFound, apperr.Conflict, apperr.RateLimited, apperr.Canceled:
		fallthrough
	default:
		return 1
	}
}

func causalDescriptorCondition(cause, outer Descriptor, known bool) bool {
	return known && cause.Code == "internal.unknown" && classificationRank(outer.Kind) >= classificationRank(cause.Kind)
}
