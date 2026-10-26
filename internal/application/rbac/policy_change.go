package rbac

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// Serializes read-modify-save sequences across all RBAC services, including compensation.
var policyChanges sync.Mutex

type policySnapshot struct{ p, g [][]string }

func snapshotPolicy(e Enforcer) (policySnapshot, error) {
	p, err := e.GetPolicy()
	if err != nil {
		return policySnapshot{}, err
	}

	g, err := e.GetNamedGroupingPolicy("g")
	if err != nil {
		return policySnapshot{}, err
	}

	clone := func(rows [][]string) [][]string {
		out := make([][]string, len(rows))
		for i, r := range rows {
			out[i] = append([]string(nil), r...)
		}

		return out
	}

	return policySnapshot{clone(p), clone(g)}, nil
}

func restorePolicy(e Enforcer, s policySnapshot) error {
	if restorer, ok := e.(interface {
		RestorePolicy([][]string, [][]string) error
	}); ok {
		return restorer.RestorePolicy(s.p, s.g)
	}

	current, err := snapshotPolicy(e)
	if err != nil {
		return err
	}

	var errs []error

	for _, p := range current.p {
		if len(p) >= 3 {
			_, err := e.RemovePolicy(p[0], p[1], p[2])
			errs = append(errs, err)
		}
	}

	for _, g := range current.g {
		if len(g) >= 2 {
			_, err := e.DeleteRoleForUser(g[0], g[1])
			errs = append(errs, err)
		}
	}

	for _, p := range s.p {
		if len(p) >= 3 {
			_, err := e.AddPolicy(p[0], p[1], p[2])
			errs = append(errs, err)
		}
	}

	for _, g := range s.g {
		if len(g) >= 2 {
			_, err := e.AddRoleForUser(g[0], g[1])
			errs = append(errs, err)
		}
	}

	errs = append(errs, e.SavePolicy())

	return errors.Join(errs...)
}

// PolicyMutation creates a private, locked policy view. Other authorization and
// identity writes keep using the synchronized enforcer and wait for completion.
type policyMutation interface{ BeginPolicyMutation() (Enforcer, func()) }

func beginPolicyChange(e Enforcer) (Enforcer, func(*error), error) {
	policyChanges.Lock()

	endMutation := func() {}
	if batching, ok := e.(policyMutation); ok {
		e, endMutation = batching.BeginPolicyMutation()
	}

	before, err := snapshotPolicy(e)
	if err != nil {
		endMutation()
		policyChanges.Unlock()

		return nil, nil, err
	}

	return e, func(result *error) {
		defer policyChanges.Unlock()
		defer endMutation()

		if *result == nil {
			return
		}

		current, readErr := snapshotPolicy(e)
		if readErr == nil && reflect.DeepEqual(before, current) {
			return
		}

		if err := restorePolicy(e, before); err != nil {
			*result = errors.Join(*result, fmt.Errorf("RBAC policy compensation failed: %w", err))
		}
	}, nil
}
