// Package clienterror maps error identities to stable classifications and safe client text.
package clienterror

import (
	"errors"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// Descriptor separates safe response text from internal error classification.
type Descriptor struct {
	Error   error
	Code    string
	Kind    apperr.Kind
	Message string
}

var descriptions = [][]Descriptor{
	domainaiDescriptions,
	domainchatDescriptions,
	domaincommentDescriptions,
	domaincontentopsDescriptions,
	domaindynamicDescriptions,
	domainidentityDescriptions,
	domainmediaDescriptions,
	domainnotificationDescriptions,
	domainrbacDescriptions,
	domainsocialDescriptions,
	domainvideoDescriptions,
	applicationcontentopsDescriptions,
	applicationidentityDescriptions,
	applicationnotificationDescriptions,
	applicationprovisioningDescriptions,
	applicationsearchDescriptions,
	applicationvideoDescriptions,
}

func lookup(err error) (Descriptor, bool) {
	for _, group := range descriptions {
		for _, d := range group {
			if errors.Is(err, d.Error) {
				return d, true
			}
		}
	}

	return Descriptor{}, false
}

// Describe gives explicit outer semantics precedence over business defaults.
func Describe(err error) Descriptor {
	d, ok := lookup(err)
	if !ok {
		d = Descriptor{Code: "internal.unknown", Kind: apperr.Internal, Message: "操作失败"}
	}

	if semantic := apperr.Semantic(err); semantic != nil {
		d.Kind = semantic.Kind
		if semantic.Code != "" {
			d.Code = semantic.Code
		} else if !ok {
			d.Code = "request." + string(semantic.Kind)
		}

		return d
	}

	if cause, exists := causalDescriptor(err); exists {
		return cause
	}

	return d
}

// Message is a pure conversion: safe visibility never controls whether an error is logged.
func Message(err error) string {
	d, _ := lookup(err)
	if d.Message == "" {
		return "操作失败"
	}

	return d.Message
}
