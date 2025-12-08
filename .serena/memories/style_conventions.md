# 代码风格与约定
- Go：使用 `go fmt`、tab 缩进；导出符号 UpperCamelCase，局部 lowerCamelCase；JSON 标签 snake_case；错误处理 `if err != nil` 并返回带上下文的 `fmt.Errorf` 信息。
- 前端：React 组件 PascalCase，目录推荐 kebab-case；TypeScript + ESLint 规范，提交前可运行 `pnpm run lint`；Tailwind 用于样式。
- 提交信息：遵循 Conventional Commits（如 `feat:` `fix:`），聚焦单一问题。
- 文档/注释：遵循现有语言（中/英混合），保持简洁；避免非 ASCII 字符 unless 已存在。
- 设计原则：遵守 SOLID/KISS/DRY/YAGNI；关注单一职责、复用、避免过度设计。