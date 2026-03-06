package transform

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "update golden fixtures")

func TestGolden_RequestConversions(t *testing.T) {
	t.Parallel()

	styles := []string{"openai", "openai-res", "anthropic"}
	for _, from := range styles {
		from := from
		t.Run(from, func(t *testing.T) {
			t.Parallel()

			in := mustReadFile(t, filepath.Join("testdata", "golden", "request", "in", from+".json"))
			for _, to := range styles {
				if to == from {
					continue
				}
				to := to
				t.Run("to_"+to, func(t *testing.T) {
					tm := NewTransformerManager(from, to)
					out, err := tm.ProcessRequest(context.Background(), in)
					if err != nil {
						t.Fatalf("ProcessRequest(%s->%s) failed: %v", from, to, err)
					}

					wantPath := filepath.Join("testdata", "golden", "request", "out", from+"_to_"+to+".json")
					assertGoldenJSON(t, wantPath, out)
				})
			}
		})
	}
}

func TestGolden_ResponseConversions(t *testing.T) {
	t.Parallel()

	styles := []string{"openai", "openai-res", "anthropic"}
	for _, from := range styles {
		from := from
		t.Run(from, func(t *testing.T) {
			t.Parallel()

			in := mustReadFile(t, filepath.Join("testdata", "golden", "response", "in", from+".json"))
			for _, to := range styles {
				if to == from {
					continue
				}
				to := to
				t.Run("to_"+to, func(t *testing.T) {
					resp := &http.Response{
						StatusCode: 200,
						Header: http.Header{
							"Content-Type": []string{"application/json"},
						},
						Body: io.NopCloser(bytes.NewReader(in)),
					}

					outResp, err := TransformProviderResponse(resp, from, to)
					if err != nil {
						t.Fatalf("TransformProviderResponse(%s->%s) failed: %v", from, to, err)
					}
					defer outResp.Body.Close()
					out, err := io.ReadAll(outResp.Body)
					if err != nil {
						t.Fatalf("read response body: %v", err)
					}

					wantPath := filepath.Join("testdata", "golden", "response", "out", from+"_to_"+to+".json")
					assertGoldenJSON(t, wantPath, out)
				})
			}
		})
	}
}

func TestGolden_StreamConversions(t *testing.T) {
	t.Parallel()

	styles := []string{"openai", "openai-res", "anthropic"}
	for _, from := range styles {
		from := from
		t.Run(from, func(t *testing.T) {
			t.Parallel()

			inDir := filepath.Join("testdata", "golden", "stream", "in")
			entries, err := os.ReadDir(inDir)
			if err != nil {
				t.Fatalf("readdir %s: %v", inDir, err)
			}

			type streamCase struct {
				name string
				in   []byte
			}
			var cases []streamCase
			for _, ent := range entries {
				if ent.IsDir() {
					continue
				}
				name := ent.Name()
				if !strings.HasSuffix(name, ".sse") {
					continue
				}

				caseName := ""
				if name == from+".sse" {
					caseName = ""
				} else if strings.HasPrefix(name, from+"__") {
					caseName = strings.TrimSuffix(strings.TrimPrefix(name, from+"__"), ".sse")
				} else {
					continue
				}

				in := mustReadFile(t, filepath.Join(inDir, name))
				cases = append(cases, streamCase{name: caseName, in: in})
			}
			if len(cases) == 0 {
				t.Fatalf("no stream fixtures found for %s in %s", from, inDir)
			}

			for _, to := range styles {
				if to == from {
					continue
				}
				to := to
				for _, c := range cases {
					c := c
					name := "to_" + to
					if c.name != "" {
						name += "__" + c.name
					}
					t.Run(name, func(t *testing.T) {
						resp := &http.Response{
							StatusCode: 200,
							Header: http.Header{
								"Content-Type": []string{"text/event-stream"},
							},
							Body: io.NopCloser(bytes.NewReader(c.in)),
						}

						outResp, err := TransformProviderResponse(resp, from, to)
						if err != nil {
							t.Fatalf("TransformProviderResponse(%s->%s) failed: %v", from, to, err)
						}
						defer outResp.Body.Close()
						out, err := io.ReadAll(outResp.Body)
						if err != nil {
							t.Fatalf("read stream body: %v", err)
						}

						wantName := from + "_to_" + to + ".sse"
						if c.name != "" {
							wantName = from + "_to_" + to + "__" + c.name + ".sse"
						}
						wantPath := filepath.Join("testdata", "golden", "stream", "out", wantName)
						assertGoldenSSE(t, wantPath, out)
					})
				}
			}
		})
	}
}

func assertGoldenJSON(t *testing.T, path string, got []byte) {
	t.Helper()

	normalized, err := normalizeJSON(got)
	if err != nil {
		t.Fatalf("normalize json: %v", err)
	}

	if *updateGolden {
		mustWriteFile(t, path, normalized)
		return
	}

	want := mustReadFile(t, path)
	if !bytes.Equal(want, normalized) {
		t.Fatalf("golden mismatch for %s\n\n--- want ---\n%s\n--- got ---\n%s", path, string(want), string(normalized))
	}
}

func assertGoldenSSE(t *testing.T, path string, got []byte) {
	t.Helper()

	normalized := normalizeSSE(got)
	if *updateGolden {
		mustWriteFile(t, path, []byte(normalized))
		return
	}

	want := string(mustReadFile(t, path))
	want = normalizeSSE([]byte(want))
	if want != normalized {
		t.Fatalf("golden mismatch for %s\n\n--- want ---\n%s\n--- got ---\n%s", path, want, normalized)
	}
}

func normalizeJSON(input []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(input, &v); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(b) > 0 && b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	return b, nil
}

func normalizeSSE(input []byte) string {
	s := string(input)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func mustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
