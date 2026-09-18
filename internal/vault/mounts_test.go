package vault

import "testing"

func TestKVVersionDetection(t *testing.T) {
	tests := []struct {
		mountType string
		options   map[string]string
		want      int
	}{
		{"kv", map[string]string{"version": "2"}, KV2},
		{"kv", map[string]string{"version": "1"}, KV1},
		{"kv", nil, KV1},
		{"cubbyhole", nil, KV1},
		{"pki", nil, KVUnknown},
		{"database", nil, KVUnknown},
	}

	for _, tc := range tests {
		if got := kvVersionOf(tc.mountType, tc.options); got != tc.want {
			t.Errorf("%s%v: got %d, want %d", tc.mountType, tc.options, got, tc.want)
		}
	}
}
