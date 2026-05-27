package auth

import "testing"

func TestPreferredUsernameExtractsJWTClaim(t *testing.T) {
	token := "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ1c3ItMSIsInByZWZlcnJlZF91c2VybmFtZSI6ImRhdmlkIn0."

	if got := preferredUsername(token); got != "david" {
		t.Fatalf("preferredUsername() = %q, want %q", got, "david")
	}
}

func TestPreferredUsernameReturnsEmptyForInvalidToken(t *testing.T) {
	if got := preferredUsername("not-a-jwt"); got != "" {
		t.Fatalf("preferredUsername() = %q, want empty", got)
	}
}
