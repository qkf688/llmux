// 图片识别能力测试用图：64x64 纯红色（#FF0000）PNG 的 data URL。
// 与后端 providers/meta_bodies.go 的 visionTestImageBase64 是同一张图（两侧改动必须同步），
// 生成脚本 .local/tools/gen_vision_test_image.py。测试体由后端构造，这里只内嵌一份供弹窗预览——
// 测试图不经 API 回显，用户选中该测试类型时即可看到「会给模型看什么图」。
export const visionTestImageDataURL =
  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAYAAACqaXHeAAAAYklEQVR42u3QMREAAAgAoe9fWnN4MlCApuazBAgQIECAAAECBAgQIECAAAECBAgQIECAAAECBAgQIECAAAECBAgQIECAAAECBAgQIECAAAECBAgQIECAAAECBAgQIECAgPsWQ4jh0jwfLk0AAAAASUVORK5CYII=";
