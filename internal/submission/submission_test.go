package submission_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ekm/mailbot/internal/submission"
)

var fixedTime = time.Date(2026, 3, 31, 14, 5, 22, 0, time.UTC)

func TestNew_TrimsWhitespace(t *testing.T) {
	s := submission.New("  Jane  ", "  jane@example.com  ", " +61400000000 ", "  Hi  ", "  body  ", "  reason  ", fixedTime)
	if s.Name != "Jane" {
		t.Errorf("Name = %q, want %q", s.Name, "Jane")
	}
	if s.Email != "jane@example.com" {
		t.Errorf("Email = %q, want %q", s.Email, "jane@example.com")
	}
	if s.Phone != "+61400000000" {
		t.Errorf("Phone = %q, want %q", s.Phone, "+61400000000")
	}
}

func TestNew_Immutable(t *testing.T) {
	s := submission.New("Jane", "jane@example.com", "", "Hi", "", "", fixedTime)
	// Reassigning the local variable does not mutate the original.
	// This test documents intent: Submission is a value type.
	other := s
	other.Name = "Bob"
	if s.Name != "Jane" {
		t.Error("Submission is not immutable: modifying copy changed original")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		sub      submission.Submission
		wantKeys []string // nil means valid; non-nil lists expected error keys
	}{
		{
			name: "valid: email and subject",
			sub:  submission.New("Jane", "jane@example.com", "", "Hello", "", "", fixedTime),
		},
		{
			name: "valid: phone only and body only",
			sub:  submission.New("", "", "+61400000000", "", "Some body", "", fixedTime),
		},
		{
			name: "valid: email and reason only",
			sub:  submission.New("", "jane@example.com", "", "", "", "support", fixedTime),
		},
		{
			name: "valid: both email and phone with all content fields",
			sub:  submission.New("Jane", "jane@example.com", "+61400000000", "Hi", "body", "reason", fixedTime),
		},
		{
			name:     "invalid: no contact info",
			sub:      submission.New("Jane", "", "", "Hello", "", "", fixedTime),
			wantKeys: []string{"email"},
		},
		{
			name:     "invalid: malformed email",
			sub:      submission.New("Jane", "not-an-email", "", "Hello", "", "", fixedTime),
			wantKeys: []string{"email"},
		},
		{
			name:     "invalid: email with missing TLD",
			sub:      submission.New("Jane", "user@", "", "Hello", "", "", fixedTime),
			wantKeys: []string{"email"},
		},
		{
			name:     "invalid: no content fields",
			sub:      submission.New("Jane", "jane@example.com", "", "", "", "", fixedTime),
			wantKeys: []string{"subject"},
		},
		{
			name:     "invalid: both contact and content missing",
			sub:      submission.New("", "", "", "", "", "", fixedTime),
			wantKeys: []string{"email", "subject"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := submission.Validate(tt.sub)
			if tt.wantKeys == nil {
				if errs != nil {
					t.Errorf("expected no errors, got: %v", errs)
				}
				return
			}
			if errs == nil {
				t.Fatal("expected validation errors, got nil")
			}
			for _, key := range tt.wantKeys {
				if _, ok := errs[key]; !ok {
					t.Errorf("expected error key %q, got errors: %v", key, errs)
				}
			}
		})
	}
}

func TestFormat_ContainsAllFields(t *testing.T) {
	s := submission.New("Jane Smith", "jane@example.com", "+61400000000", "Website inquiry", "Hello there", "Support", fixedTime)
	got := submission.Format(s)

	mustContain := []string{
		"Date:    2026-03-31T14:05:22Z",
		"Name:    Jane Smith",
		"Email:   jane@example.com",
		"Phone:   +61400000000",
		"Subject: Website inquiry",
		"Reason:  Support",
		"Body:\nHello there",
	}
	for _, want := range mustContain {
		if !strings.Contains(got, want) {
			t.Errorf("Format() missing %q\nFull output:\n%s", want, got)
		}
	}
}

func TestFormat_EmptyBodyOmitted(t *testing.T) {
	s := submission.New("", "jane@example.com", "", "Hello", "", "", fixedTime)
	got := submission.Format(s)
	if strings.Contains(got, "Body:") {
		t.Errorf("Format() should omit Body section when body is empty\nGot:\n%s", got)
	}
}

func TestFilename_Format(t *testing.T) {
	s := submission.New("", "jane@example.com", "", "Hi", "", "", fixedTime)
	name := submission.Filename(s)
	pattern := regexp.MustCompile(`^\d{8}-\d{6}-[a-z0-9]{6}\.txt$`)
	if !pattern.MatchString(name) {
		t.Errorf("Filename() = %q, does not match YYYYMMDD-HHMMSS-xxxxxx.txt", name)
	}
}

func TestFilename_UsesUTC(t *testing.T) {
	s := submission.New("", "jane@example.com", "", "Hi", "", "", fixedTime)
	name := submission.Filename(s)
	if !strings.HasPrefix(name, "20260331-140522-") {
		t.Errorf("Filename() = %q, expected prefix 20260331-140522-", name)
	}
}

func TestFilename_Unique(t *testing.T) {
	s := submission.New("", "jane@example.com", "", "Hi", "", "", fixedTime)
	names := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		n := submission.Filename(s)
		names[n] = struct{}{}
	}
	// With 36^6 ≈ 2.2B combinations, 100 calls should always be unique.
	if len(names) != 100 {
		t.Errorf("Filename() produced %d duplicates in 100 calls", 100-len(names))
	}
}
