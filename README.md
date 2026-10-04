# miniDocker

Container runtime viết từ đầu bằng Go, không dùng Docker hay runc — tự tay dựng lại các cơ chế kernel Linux mà container thật sự dùng: namespaces, chroot, cgroups, và network bằng veth pair.

Dự án làm để hiểu "container hoạt động ra sao bên dưới" chứ không phải để thay thế Docker.

## Tính năng

- **Namespace cô lập**: UTS (hostname), PID (cây process riêng), Mount (filesystem riêng), Network (card mạng riêng)
- **Chroot** vào root filesystem riêng (Alpine Linux minirootfs)
- **cgroup v2**: giới hạn RAM qua `memory.max`
- **Network namespace thật**: veth pair + NAT (iptables MASQUERADE), container ra internet thật qua địa chỉ IP riêng
- **CLI**: `run`, `ps`, `stop` với cờ `--name`, `--memory`
- Lưu trạng thái container ra JSON, dọn dẹp (cgroup, veth, state) khi `stop`

## Kiến trúc

```
minidocker run --name web1 --memory 100 /bin/sh
        │
        ▼
   parent()  ── parse cờ --name/--memory
        │
        ├─ exec.Command("/proc/self/exe", "child", name, cmd...)
        │  với Cloneflags: CLONE_NEWUTS | CLONE_NEWPID | CLONE_NEWNS | CLONE_NEWNET
        │
        ├─ setupNetwork(pid, name)   — tạo veth pair, gán IP host, NAT
        ├─ setupCgroup(pid, memMB)   — giới hạn RAM qua /sys/fs/cgroup
        ├─ saveContainerInfo(...)    — ghi JSON state
        │
        ▼
   child()  (chạy trong namespace mới, tự động là PID 1)
        │
        ├─ Sethostname()
        ├─ Chroot() + Chdir("/")     — đổi root filesystem
        ├─ Mount("/proc")            — cho ps hoạt động đúng
        ├─ setupContainerNetwork()   — gán IP, route bên trong container
        │
        └─ syscall.Exec(binary, args, env)  — thay thế chính mình bằng lệnh thật
```

**Điểm kỹ thuật đáng chú ý:**
- Dùng pattern `/proc/self/exe` để tự gọi lại chính mình làm process con — cách Go lách việc không có `fork()` an toàn trong chương trình multi-thread
- `syscall.Exec` (không phải `cmd.Run()`) ở tầng child, để quá trình cuối cùng giữ đúng PID 1 trong namespace, thay vì trở thành PID 2
- Network dùng thư viện `netlink` gọi thẳng kernel qua netlink socket, không shell ra lệnh `ip` — đúng cách runc/Docker thật làm

## Cách chạy

Yêu cầu: Linux (hoặc WSL2), Go 1.21+, quyền root.

```bash
go build -o minidocker .

# Tải Alpine rootfs (1 lần)
mkdir -p ~/mydocker-rootfs && cd ~/mydocker-rootfs
curl -O https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/x86_64/alpine-minirootfs-3.21.0-x86_64.tar.gz
tar -xzf alpine-minirootfs-*.tar.gz

cd -
sudo ./minidocker run --name web1 --memory 100 /bin/sh
```

Trong container:

```bash
hostname          # minicontainer — hostname riêng
echo $$           # 1 — PID namespace riêng
ping 8.8.8.8      # ra internet qua veth + NAT
```

Ở terminal khác:

```bash
sudo ./minidocker ps
sudo ./minidocker stop web1
```

## Benchmark

So sánh trên cùng máy (WSL2, Ubuntu), Docker đã cài sẵn và image đã pull trước:

| Phép đo | minidocker | docker run |
|---|---|---|
| Thời gian khởi động (`echo hello`) | 0.273s | 0.680s |

`minidocker` khởi động nhanh hơn Docker thật khoảng 2.5 lần trong phép đo này — hợp lý vì Docker đi qua nhiều lớp daemon (`dockerd` → `containerd` → `containerd-shim` → `runc`), còn `minidocker` chỉ là 1 binary gọi thẳng syscall, không qua IPC/daemon nào.

*Lưu ý: đây là phép đo đơn giản trên 1 máy, không đại diện cho hiệu năng production. Docker có nhiều việc phải làm hơn (logging driver, network plugin, volume mount...) mà minidocker không có.*

## Hạn chế đã biết

So với Docker/runc thật, minidocker còn thiếu:

- **Chỉ chạy 1 container tại 1 thời điểm** — IP trong container đang hardcode `10.0.0.2`, chạy 2 container cùng lúc sẽ đụng IP. Docker thật dùng IPAM (IP Address Management) để cấp phát động.
- **Không có `/dev`** — chưa mount hay tạo device node (`/dev/zero`, `/dev/null`, `/dev/urandom`...), nên một số chương trình cần các file này sẽ lỗi.
- **Không có image layer** — dùng thẳng 1 bản Alpine rootfs copy cứng, không có overlay filesystem, không cache layer, không `docker build`.
- **Không có user namespace** (`CLONE_NEWUSER`) — container chạy với quyền root thật của host, không cô lập quyền.
- **Không có DNS riêng** — container dùng chung resolver của host (qua file `/etc/resolv.conf` kế thừa, chưa tự quản lý).
- **cgroup chỉ giới hạn RAM** — chưa có `--cpus`, `pids.max`, I/O limit.

## Những gì học được

Phát hiện thú vị nhất trong quá trình làm: máy có cài Docker thật sẽ tự đặt `iptables FORWARD` policy thành `DROP` để kiểm soát traffic container của chính nó — khiến traffic của `minidocker` (dùng dải IP khác) bị chặn hoàn toàn dù route/NAT đều đúng, cho tới khi thêm rule `ACCEPT` tường minh cho dải IP của minidocker. Đây là ví dụ thực tế về việc 2 container runtime khác nhau có thể xung đột ở tầng mạng khi chạy chung 1 kernel.
