package balancer

import "testing"

func TestAffinityKey_StableAndDistinct(t *testing.T) {
	sys := [][]byte{[]byte(`{"system":"s"}`)}
	base := AffinityKey("m", sys, []byte(`[{"name":"t"}]`), []byte(`{"role":"user","content":"hi"}`))

	same := AffinityKey("m", [][]byte{[]byte(`{"system":"s"}`)}, []byte(`[{"name":"t"}]`), []byte(`{"role":"user","content":"hi"}`))
	if same != base {
		t.Fatalf("same semantic parts produced different keys: %q vs %q", same, base)
	}

	cases := map[string]struct {
		model        string
		system       [][]byte
		tools        []byte
		firstMessage []byte
	}{
		"model changed":            {"other", sys, []byte(`[{"name":"t"}]`), []byte(`{"role":"user","content":"hi"}`)},
		"system changed":           {"m", [][]byte{[]byte(`{"system":"S"}`)}, []byte(`[{"name":"t"}]`), []byte(`{"role":"user","content":"hi"}`)},
		"tools changed":            {"m", sys, []byte(`[{"name":"u"}]`), []byte(`{"role":"user","content":"hi"}`)},
		"first message changed":    {"m", sys, []byte(`[{"name":"t"}]`), []byte(`{"role":"user","content":"bye"}`)},
		"system seg count changed": {"m", [][]byte{[]byte(`{"system":"s"}`), nil}, []byte(`[{"name":"t"}]`), []byte(`{"role":"user","content":"hi"}`)},
		"parts shifted":            {"m", [][]byte{[]byte(`{"system":"s"}x`)}, nil, []byte(`{"role":"user","content":"hi"}`)},
	}
	for name, c := range cases {
		got := AffinityKey(c.model, c.system, c.tools, c.firstMessage)
		if got == base {
			t.Fatalf("%s: key collided with base", name)
		}
	}

	// 长度前缀防拼接边界撞车：system 相邻两段「ab|c」与「a|bc」朴素拼接同形，键必须不同（防漂移锁）。
	if AffinityKey("m", [][]byte{[]byte("ab"), []byte("c")}, nil, nil) == AffinityKey("m", [][]byte{[]byte("a"), []byte("bc")}, nil, nil) {
		t.Fatal("system segments shifted across boundary collided")
	}
}
