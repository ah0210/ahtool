# frontend/dist —— Wails 资产占位目录

这个目录会被编译进程序（`main.go` 里 `//go:embed all:frontend/dist`）。

## `index.html` 必须保留

Wails v2 的 `assetserver` 在启动时会检查 embed 资产里是否存在 `index.html`；
没有就直接返回：

```
no `index.html` could be found in your Assets fs.FS
```

GUI 子系统没有控制台，这个错误会表现为“双击无反应”。`index.html` 只是启动占位，
真正的页面由 `main.go` 的 `serveDesignPage` 中间件返回 `design/ui-prototype.html`。
不要把本文件删掉。

## 其他素材

本分支不再使用服务方二维码 / 头像 / 赞赏码。如需增加自己的静态资源，放进本目录，
并在 `main.go` 的 `serveDesignPage` 中间件里按需提供。
