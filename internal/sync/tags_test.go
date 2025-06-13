package sync

import "testing"

func TestIsSemVerTag(t *testing.T) {
	tests := []struct {
		tag      string
		expected bool
	}{
		{"1.0.0", true},
		{"v1.0.0", true},
		{"1.0.0-alpha", true},
		{"1.0.0+build", true},
		{"latest", false},
		{"1.0", true},
		{"1.0.0-beta+exp.sha.5114f85", true},
		{"25.3.3.20143.altinityantalya", false},
		{"25.3.3-altinityantalya.20143", true},
	}

	for _, test := range tests {
		result := isSemVerTag(test.tag)
		if result != test.expected {
			t.Errorf("isSemVerTag(%q) = %v; want %v", test.tag, result, test.expected)
		}
	}
}
