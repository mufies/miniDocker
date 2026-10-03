package main

import (
	"fmt"
	"net"
	"os/exec"
	"time"

	"github.com/vishvananda/netlink"
)

func setupNetwork(pid int) error {
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: "veth-host"},
		PeerName:  "veth-ns",
	}
	err := netlink.LinkAdd(veth)
	if err != nil {
		return err
	}

	vethNs, err := netlink.LinkByName("veth-ns")
	if err != nil {
		return fmt.Errorf("Loi tim veth-ns: %w", err)
	}

	err = netlink.LinkSetNsPid(vethNs, pid)
	if err != nil {
		return fmt.Errorf("Loi dua veth-ns vao namespace: %w", err)
	}

	// gan ip cho host
	vethHost, err := netlink.LinkByName("veth-host")
	if err != nil {
		return fmt.Errorf("lỗi gán IP veth-host: %w", err)
	}
	addr, err := netlink.ParseAddr("10.0.0.1/24")
	if err != nil {
		return fmt.Errorf("lỗi parse IP: %w", err)
	}
	err = netlink.AddrAdd(vethHost, addr)
	if err != nil {
		return fmt.Errorf("lỗi gán IP veth-host: %w", err)
	}

	err = netlink.LinkSetUp(vethHost)
	if err != nil {
		return fmt.Errorf("Ko up duoc host: %w", err)
	}

	cmd := exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING",
		"-s", "10.0.0.0/24", "-o", "enP29578p0s0", "-j", "MASQUERADE")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("lỗi iptables: %w", err)
	}

	return nil
}

func setupContainerNetwork() error {
	// Đợi 1 chút để đảm bảo namespace đã sẵn sàng (tránh race condition)
	time.Sleep(100 * time.Millisecond)

	link, err := netlink.LinkByName("veth-ns")
	if err != nil {
		return fmt.Errorf("lỗi tìm veth-ns: %w", err)
	}

	addr, _ := netlink.ParseAddr("10.0.0.2/24")
	err = netlink.AddrAdd(link, addr)
	if err != nil {
		return fmt.Errorf("lỗi gán IP veth-ns: %w", err)
	}

	err = netlink.LinkSetUp(link)
	if err != nil {
		return fmt.Errorf("lỗi up veth-ns: %w", err)
	}

	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("lỗi tìm lo: %w", err)
	}
	err = netlink.LinkSetUp(lo)
	if err != nil {
		return fmt.Errorf("lỗi up lo: %w", err)
	}

	route := &netlink.Route{
		Scope: netlink.SCOPE_UNIVERSE,
		Gw:    net.ParseIP("10.0.0.1"),
	}
	err = netlink.RouteAdd(route)
	if err != nil {
		return fmt.Errorf("lỗi thêm default route: %w", err)
	}

	return nil
}
