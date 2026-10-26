// 文件用途：验证 app 会员使用平台级版本 key，不依赖商户 ID。

package authentication

import "testing"

// TestBuildTokenVersionKeyAppDoesNotRequireShop 验证 app 会员使用平台级版本 key，不依赖商户 ID。
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
