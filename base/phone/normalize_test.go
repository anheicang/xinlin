package phone

import "testing"

func TestNormalize(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"151 9009 5877", "15190095877"},
		{"\t151\u30009009\u00a05877\r\n", "15190095877"},
		{"151\u202f9009\u00855877\ufeff", "15190095877"},
		{"151-9009-5877", "151-9009-5877"},
		{"151a90095877", "151a90095877"},
		{" \t\n", ""},
		{"+86 15190095877", "+8615190095877"},
	} {
		if got := Normalize(tc.input); got != tc.want {
			t.Errorf("Normalize(%q)=%q want %q", tc.input, got, tc.want)
		}
	}
}

func TestNormalizeSearch(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"151 9009 5877", "15190095877"},
		{" 张 三 ", "张 三"},
		{" account name ", "account name"},
	} {
		if got := NormalizeSearch(tc.input); got != tc.want {
			t.Errorf("NormalizeSearch(%q)=%q want %q", tc.input, got, tc.want)
		}
	}
}
