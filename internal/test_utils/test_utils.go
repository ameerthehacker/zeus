package test_utils

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/error"
)

func CompareZeusErrors(t *testing.T, errors, expected []*error.ZeusError) {
	if len(errors) > 0 && len(expected) == 0 {
		t.Error("expected errors but got none")
	} else if len(errors) == 0 && len(expected) > 0 {
		t.Errorf("unexpected errors: %v", expected)
	} else if len(errors) != len(expected) {
		t.Errorf("expected %d errors, got %d", len(errors), len(expected))
	} else {
		for i, error := range expected {
			expected := errors[i]
			if !error.IsEqual(expected) {
				t.Errorf("error %s: expected %s, got %s", error, expected, error)
			}
		}
	}
}
