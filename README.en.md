# ah System Toolbox (ahtool) — Windows Cleaner & Maintenance Suite

A single-executable Windows desktop tool built with Go + Wails v2: junk cleanup,
app uninstall with residue removal, system tweaks, startup management, disk
space analysis, driver export, blue-screen dump parsing, hardware info, and 36
built-in system tools.

- Version: **v1.0.0** (fork baseline, based on upstream v1.1.0, GPLv3)
- Platform: Windows 10 / 11 · amd64
- Distribution: portable single exe, requests administrator at launch
- Frontend: plain HTML/CSS/JS embedded with `go:embed`, no Node build step

> This fork removes the original author's server bindings: online update is
> **disabled by default**, and there are no website / community / donation /
> remote-service links. See [`NOTICE`](NOTICE).

## Networking

The default build performs no network request at startup and contains no
telemetry or account system. The only on-demand networking is the user-triggered
**Speed Test**, which contacts public mirrors / Cloudflare / Ookla endpoints to
measure download, upload, latency and packet loss. The upstream plaintext-HTTP
public-IP lookup has been removed.

The update code remains in `update.go` but is disabled (`updateEnabled=false`,
empty manifest URL). A fork maintainer can point it at their own HTTPS endpoint.

## Build

Requires Go 1.26+.

```bat
build.cmd            :: dev build
build.cmd release    :: bin\ahtool_v1.0.0.exe
```

```powershell
pwsh -File build.ps1 -Release
```

```bash
GOPROXY=https://goproxy.cn,direct ./build.sh release
```

The version is defined once as `APPVER` in the build scripts and injected via
`-X main.appVersion` / `-X main.buildDate`. The frontend reads it from
`GetAboutInfo()`.

## Test

```bash
go test ./...                # safe, no real-disk scans
go test -tags manual ./...   # real-machine / admin tests
```

## License

GPLv3. Original copyright notices are retained in [`NOTICE`](NOTICE). The
original author's name, logo and donation QR code are not covered by the license
and have been replaced with neutral branding.
