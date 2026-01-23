package main

import (
	"embed"
	"io/fs"
)

// StaticFiles 嵌入的前端静态文件
//
//go:embed web/dist/*
var StaticFiles embed.FS

// GetStaticFS 获取静态文件系统，返回 web/dist 子目录
func GetStaticFS() (fs.FS, error) {
	return fs.Sub(StaticFiles, "web/dist")
}
