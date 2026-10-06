# UML on Windows (via WSL2)

User-Mode Linux kernels are Linux ELF binaries — they cannot run on the
Windows kernel directly. The Windows edition of this launcher therefore
runs the whole stack (kernel + `vdeplug-netstack` engine + cloud image)
inside **WSL2**, driven by a native PowerShell entry point. From the
user's side it is one command, exactly like on Linux.

## Requirements

- Windows 10 21H2+ / Windows 11 with **WSL2** (`wsl --install`, reboot once)
- Any WSL2 distro (Ubuntu default is fine) — no packages to install inside
  it; the kernel, engine and image are self-contained
- The release files in one folder, e.g. `C:\uml\`:
  `boot.ps1`, `boot`, `linux`, `base.img`, `vdeplug-netstack`,
  optionally `config.yaml`

> Put the folder on a normal Windows drive (`C:\uml`). WSL2 maps it as
> `/mnt/c/uml`. It also works if the files already live inside the WSL
> filesystem (`\\wsl$\Ubuntu\home\you\uml`) — the launcher detects both.

## Quick start

```powershell
cd C:\uml
.\boot.ps1            # config.yaml defaults (2G RAM, 1 vCPU)
.\boot.ps1 4G 2       # 4 GB RAM, 2 vCPUs
.\boot.ps1 -Distro Ubuntu-24.04   # pick a non-default distro
```

If PowerShell blocks the script, allow local scripts once:

```powershell
Set-ExecutionPolicy -Scope CurrentUser RemoteSigned
```

Login is `root` / `root`, same as on Linux.

## Networking on Windows

- NAT (slirp) needs no privileges and works out of the box in WSL2.
- **Port forwards**: `ports: ["2222:22"]` binds inside WSL2. WSL2's
  localhost forwarding (on by default) exposes it to Windows, so
  `ssh root@localhost -p 2222` works from PowerShell/CMD directly.
  To reach the VM from *other* machines, add a Windows port proxy:

  ```powershell
  netsh interface portproxy add v4tov4 listenport=2222 listenaddress=0.0.0.0 connectport=2222 connectaddress=localhost
  ```

- The multi-instance switch works between VMs booted on the **same** WSL2
  distro; the switch socket lives at `/tmp/vde.socket` inside WSL.
- `uplink: tap:NAME` requires `/dev/net/tun` in WSL2 (available on recent
  WSL2 kernels) — but bridging into the real LAN is a WSL2/Hyper-V
  networking topic of its own; prefer `slirp` on Windows.

## What the launcher does

`boot.ps1` locates its own folder, verifies WSL2 and the required files,
translates the folder to a WSL path (`wslpath`), restores exec bits lost
when the files were copied through Windows (`chmod +x linux
vdeplug-netstack boot`), then hands over to the regular `boot` script via
`wsl --cd`. Memory/vCPU arguments and `config.yaml` behave exactly as on
Linux — see `config.example.yaml`.
