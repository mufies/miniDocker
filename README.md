# miniDocker

A container runtime built from scratch in Go — no Docker, no runc. It directly implements the Linux kernel mechanisms that real containers rely on: namespaces, chroot, cgroups, and networking via veth pairs.

This is a research/educational project exploring how container isolation actually works at the kernel level. It is not intended as a production-ready or Docker-replacement tool.

## Features

- **Namespace isolation**: UTS (hostname), PID (separate process tree), Mount (separate filesystem view), Network (separate network stack)
- **Chroot** into an isolated root filesystem (Alpine Linux minirootfs)
- **cgroup v2**: memory limiting via `memory.max`
- **Real network namespace**: veth pair + NAT (iptables MASQUERADE), giving the container real internet access through its own IP
- **CLI**: `run`, `ps`, `stop` with `--name` and `--memory` flags
- Container state persisted to JSON; cleanup (cgroup, veth, state) handled on `stop`

## Architecture

```
minidocker run --name web1 --memory 100 /bin/sh
        │
        ▼
   parent()  ── parses --name/--memory flags
        │
        ├─ exec.Command("/proc/self/exe", "child", name, cmd...)
        │  with Cloneflags: CLONE_NEWUTS | CLONE_NEWPID | CLONE_NEWNS | CLONE_NEWNET
        │
        ├─ setupNetwork(pid, name)   — creates veth pair, assigns host IP, sets up NAT
        ├─ setupCgroup(pid, memMB)   — applies memory limit via /sys/fs/cgroup
        ├─ saveContainerInfo(...)    — writes JSON state
        │
        ▼
   child()  (runs inside the new namespaces, automatically becomes PID 1)
        │
        ├─ Sethostname()
        ├─ Chroot() + Chdir("/")     — switches root filesystem
        ├─ Mount("/proc")            — required for ps to work correctly
        ├─ setupContainerNetwork()   — assigns IP/route inside the container
        │
        └─ syscall.Exec(binary, args, env)  — replaces itself with the target process
```

**Notable implementation details:**
- Uses the `/proc/self/exe` re-exec pattern to spawn a child process, working around Go's lack of a safe `fork()` in multi-threaded programs
- Uses `syscall.Exec` (not `cmd.Run()`) in the child, so the final process correctly becomes PID 1 inside the namespace instead of PID 2
- Networking is implemented via the `netlink` library, talking to the kernel directly over netlink sockets rather than shelling out to `ip` — the same approach runc/Docker use internally

## Usage

Requirements: Linux (or WSL2), Go 1.21+, root privileges.

```bash
go build -o minidocker .

# Download the Alpine rootfs (one-time setup)
mkdir -p ~/mydocker-rootfs && cd ~/mydocker-rootfs
curl -O https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/x86_64/alpine-minirootfs-3.21.0-x86_64.tar.gz
tar -xzf alpine-minirootfs-*.tar.gz

cd -
sudo ./minidocker run --name web1 --memory 100 /bin/sh
```

Inside the container:

```bash
hostname          # minicontainer — isolated hostname
echo $$           # 1 — isolated PID namespace
ping 8.8.8.8      # real internet access via veth + NAT
```

From another terminal:

```bash
sudo ./minidocker ps
sudo ./minidocker stop web1
```

## Benchmark

Measured on the same machine (WSL2, Ubuntu), with Docker pre-installed and the image pre-pulled:

| Measurement | minidocker | docker run |
|---|---|---|
| Startup time (`echo hello`) | 0.273s | 0.680s |

minidocker starts roughly 2.5x faster than real Docker in this measurement — expected, since Docker goes through several daemon layers (`dockerd` → `containerd` → `containerd-shim` → `runc`), while minidocker is a single binary making direct syscalls with no IPC/daemon overhead.

*Note: this is a simple single-machine measurement, not representative of production performance. Docker handles substantially more (logging drivers, network plugins, volume mounts, etc.) that minidocker does not.*

## Known Limitations

Compared to real Docker/runc, minidocker currently lacks:

- **Single container at a time** — the in-container IP is hardcoded to `10.0.0.2`; running two containers simultaneously would collide. Real Docker uses dynamic IPAM (IP Address Management).
- **No `/dev`** — device nodes (`/dev/zero`, `/dev/null`, `/dev/urandom`, etc.) are neither mounted nor created, so programs that depend on them will fail.
- **No image layers** — uses a single hardcoded Alpine rootfs copy; no overlay filesystem, no layer caching, no `docker build` equivalent.
- **No user namespace** (`CLONE_NEWUSER`) — the container runs with the host's real root privileges; no privilege isolation.
- **No dedicated DNS** — the container inherits the host's resolver rather than managing its own.
- **cgroup limits only memory** — no `--cpus`, `pids.max`, or I/O limits yet.

## Interesting finding

The most notable discovery during development: a machine with real Docker installed sets the `iptables FORWARD` chain policy to `DROP` to control its own containers' traffic — which silently blocked minidocker's traffic (a different IP range) even though routing and NAT were both correctly configured, until an explicit `ACCEPT` rule was added for minidocker's subnet. A concrete example of how two independent container runtimes can conflict at the network layer when sharing a single kernel.
