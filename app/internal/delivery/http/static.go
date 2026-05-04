package http

import "embed"

//go:embed static/*
var embeddedStaticFiles embed.FS
