package providers

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// 测试图常量是双份内嵌（后端 meta_bodies.go + 前端 vision-test-image.ts），生成脚本在
// .local/（gitignore，不随仓库分发）——本文件把「常量没被改坏、两侧没漂移」从注释互指
// 升级成机械契约：像素内容自证 + 前后端逐字节比对。任一侧手滑都在这里拦住。

func TestVisionTestImage_PixelsAreSolidRed(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString(visionTestImageBase64)
	if err != nil {
		t.Fatalf("visionTestImageBase64 is not valid base64: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	bounds := img.Bounds().Dx()
	if bounds != 64 || img.Bounds().Dy() != 64 {
		t.Fatalf("image size = %dx%d, want 64x64 (Anthropic 要求 ≥32x32)", bounds, img.Bounds().Dy())
	}
	for x := 0; x < 64; x++ {
		for y := 0; y < 64; y++ {
			// RGBA() 返回 alpha 预乘 uint32：不透明纯红 = (0xffff, 0, 0, 0xffff)
			r, g, b, a := img.At(x, y).RGBA()
			if r != 0xffff || g != 0 || b != 0 || a != 0xffff {
				t.Fatalf("pixel(%d,%d) = (%#x,%#x,%#x,%#x), want 纯红 #FF0000", x, y, r, g, b, a)
			}
		}
	}
}

func TestVisionTestImage_FrontendConstantInSync(t *testing.T) {
	path := filepath.Join("..", "webui", "src", "routes", "model-providers", "vision-test-image.ts")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read frontend constant file: %v", err)
	}
	match := regexp.MustCompile(`data:image/png;base64,([A-Za-z0-9+/=]+)`).FindSubmatch(src)
	if match == nil {
		t.Fatalf("no data URL found in %s", path)
	}
	if string(match[1]) != visionTestImageBase64 {
		t.Fatal("frontend visionTestImageDataURL drifted from backend visionTestImageBase64（两侧必须逐字节一致）")
	}
}

// 三个协议测试体必须都内嵌同一张图：漏引用 = 某一类上游实际没在测视觉。
func TestVisionBodies_EmbedTestImage(t *testing.T) {
	for name, body := range map[string]string{
		"openai":     visionBodyOpenAI,
		"openai-res": visionBodyOpenAIRes,
		"anthropic":  visionBodyAnthropic,
	} {
		if !bytes.Contains([]byte(body), []byte(visionTestImageBase64)) {
			t.Errorf("%s vision body does not embed visionTestImageBase64", name)
		}
	}
}
