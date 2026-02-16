# docker-win-net-connect

Connect Docker Desktop networks to the Windows host by creating a WireGuard
tunnel between the host and the Docker Desktop VM and routing Docker network
subnets through it.

## What it does
- Creates a WireGuard tunnel interface named `docker-win-net-connect`.
- Uses a setup container (`wpkpda/docker-win-net-setup`) to configure the VM peer.
- Watches Docker network create/destroy events and updates host routes.
- Lets the Windows host reach container subnets (including VPN-routed subnets).

Default tunnel settings (can be changed in code):
- Host peer IP: `10.20.30.1`
- VM peer IP: `10.20.30.2`
- Port: `2030`

## Requirements
- Windows 10/11.
- Docker Desktop running.
- Administrator privileges (installs a service and modifies routes).

## Install
### Scoop
```powershell
scoop bucket add rocky https://github.com/i-rocky/bucket
scoop install docker-win-net-connect
```

### Manual
Download the latest release zip and extract `docker-win-net-connect.exe`.

## Usage
Run commands from an elevated shell:
```powershell
docker-win-net-connect.exe install
docker-win-net-connect.exe start
```

Service control:
```powershell
docker-win-net-connect.exe stop
docker-win-net-connect.exe uninstall
```

Debug (run in console, not as a service):
```powershell
docker-win-net-connect.exe debug
```

## Build
You need Go and the WireGuard binaries for your target architecture.

```powershell
go build -o dist/docker-win-net-connect.exe .
```

The binaries in `bin/` are embedded at build time:
- `bin/wg.exe`
- `bin/wireguard.exe`

## Troubleshooting
- Ensure Docker Desktop is running.
- Check Windows Event Viewer → Application log → `docker-win-net-connect`.
- Re-run `install` if the service or event log entry is missing.

## License
MIT
