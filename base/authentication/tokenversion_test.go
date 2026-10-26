package authentication

import "testing"

func TestBuildTokenVersionKeyAppDoesNotRequireShop(t *testing.T) {
	tests := []struct {
		name   string
		shopID int64
	}{
		{name: "platform member", shopID: 0},
		{name: "legacy request with shop", shopID: 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildTokenVersionKey(ClientTypeApp, tt.shopID, 123)
			if err != nil {
				t.Fatalf("buildTokenVersionKey() error = %v", err)
			}
			if want := "jv_member_user:123"; got != want {
				t.Fatalf("buildTokenVersionKey() = %q, want %q", got, want)
			}
		})
	}
}
