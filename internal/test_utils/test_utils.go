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
		t.Errorf("expected %d errors, got %d", len(expected), len(errors))
	} else {
		for i, error := range errors {
			expectedError := expected[i]
			if !error.IsEqual(expectedError) {
				t.Errorf("expected error %s, got %s", expectedError, error)
			}
		}
	}
}
