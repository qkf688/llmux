package balancer

import "testing"

func TestAffinityKey_StableAndDistinct(t *testing.T) {
	base := AffinityKey("m", []byte(`{"system":"s"}`), []byte(`[{"name":"t"}]`), []byte(`{"role":"user","content":"hi"}`))

	same := AffinityKey("m", []byte(`{"system":"s"}`), []byte(`[{"name":"t"}]`), []byte(`{"role":"user","content":"hi"}`))
	if same != base {
		t.Fatalf("same semantic parts produced different keys: %q vs %q", same, base)
	}

	cases := map[string]struct {
		model        string
		system       []byte
		tools        []byte
		firstMessage []byte
	}{
		"model changed":         {"other", []byte(`{"system":"s"}`), []byte(`[{"name":"t"}]`), []byte(`{"role":"user","content":"hi"}`)},
		"system changed":        {"m", []byte(`{"system":"S"}`), []byte(`[{"name":"t"}]`), []byte(`{"role":"user","content":"hi"}`)},
		"tools changed":         {"m", []byte(`{"system":"s"}`), []byte(`[{"name":"u"}]`), []byte(`{"role":"user","content":"hi"}`)},
		"first message changed": {"m", []byte(`{"system":"s"}`), []byte(`[{"name":"t"}]`), []byte(`{"role":"user","content":"bye"}`)},
		"parts shifted":         {"m", []byte(`{"system":"s"}x`), nil, []byte(`{"role":"user","content":"hi"}`)},
	}
	for name, c := range cases {
		got := AffinityKey(c.model, c.system, c.tools, c.firstMessage)
		if got == base {
			t.Fatalf("%s: key collided with base", name)
		}
	}

	// 长度前缀防拼接边界撞车：相邻两段「ab|c」与「a|bc」朴素拼接同形，键必须不同（防漂移锁）。
	if AffinityKey("m", []byte("ab"), []byte("c"), nil) == AffinityKey("m", []byte("a"), []byte("bc"), nil) {
		t.Fatal("parts shifted across segment boundary collided")
	}
}
